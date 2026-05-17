package sip

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anderstorpsfestivalen/benis-phone/core/callctl"
	"github.com/emiago/diago"
	"github.com/emiago/sipgo/sip"
	log "github.com/sirupsen/logrus"
)

// sipController is the per-call implementation of callctl.Controller. It
// forwards each method to the equivalent diago primitive on the live dialog,
// and delegates recording to a shared *recorder also held by SIPPhone and
// OutputStream so all three can tap the same RTP streams.
type sipController struct {
	dialog        *diago.DialogServerSession
	callID        string
	recordBase    string    // from def.SIP.RecordPath
	defaultDomain string    // from def.SIP.Domain (for transfer shorthand)
	rec           *recorder // shared with SIPPhone + OutputStream
}

var _ callctl.Controller = (*sipController)(nil)

func newSIPController(callID string, dialog *diago.DialogServerSession, recordBase, defaultDomain string, rec *recorder) *sipController {
	return &sipController{
		dialog:        dialog,
		callID:        callID,
		recordBase:    recordBase,
		defaultDomain: defaultDomain,
		rec:           rec,
	}
}

func (c *sipController) ID() string { return c.callID }

func (c *sipController) Hangup(ctx context.Context) error {
	log.WithField("call_id", c.callID).Info("Hangup requested")
	return c.dialog.Hangup(ctx)
}

func (c *sipController) Transfer(ctx context.Context, target string) error {
	uri, err := parseTransferTarget(target, c.defaultDomain)
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{"call_id": c.callID, "target": uri.String()}).Info("Transfer requested")
	return c.dialog.Refer(ctx, uri)
}

func (c *sipController) SendDTMF(digits string) error {
	if digits == "" {
		return nil
	}
	w := c.dialog.AudioWriterDTMF()
	log.WithFields(log.Fields{"call_id": c.callID, "digits": digits}).Info("Sending DTMF")
	for i, d := range digits {
		if err := w.WriteDTMF(d); err != nil {
			return fmt.Errorf("dtmf write %q: %w", string(d), err)
		}
		// Inter-digit gap (skip after last)
		if i < len(digits)-1 {
			time.Sleep(200 * time.Millisecond)
		}
	}
	return nil
}

func (c *sipController) StartRecording(subfolder string) (string, error) {
	if subfolder == "" {
		subfolder = "adhoc"
	}
	dir := filepath.Join(c.recordBase, subfolder)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("recording mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, time.Now().Format("2006-01-02_15-04-05")+".wav")
	if err := c.rec.Start(path); err != nil {
		return "", err
	}
	return path, nil
}

func (c *sipController) StopRecording() error {
	return c.rec.Stop()
}

// parseTransferTarget accepts a SIP URI ("sip:user@host[:port]"), a bare
// URI without scheme ("user@host"), or an extension shorthand ("200") in
// which case it expands using defaultDomain. Returns an error if the input
// is empty or, for the shorthand case, defaultDomain is empty.
func parseTransferTarget(target, defaultDomain string) (sip.Uri, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return sip.Uri{}, fmt.Errorf("transfer target is empty")
	}

	// Extension shorthand: no '@', no ':' → expand with defaultDomain.
	if !strings.ContainsAny(target, "@:") {
		if defaultDomain == "" {
			return sip.Uri{}, fmt.Errorf("transfer shorthand %q used without a default SIP domain", target)
		}
		target = "sip:" + target + "@" + defaultDomain
	} else if !strings.HasPrefix(target, "sip:") && !strings.HasPrefix(target, "sips:") {
		// "user@host" → prepend "sip:"
		target = "sip:" + target
	}

	var uri sip.Uri
	if err := sip.ParseUri(target, &uri); err != nil {
		return sip.Uri{}, fmt.Errorf("parse transfer target %q: %w", target, err)
	}
	return uri, nil
}
