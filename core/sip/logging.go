package sip

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/emiago/diago/media"
	sipgosip "github.com/emiago/sipgo/sip"
	"github.com/sirupsen/logrus"
)

// wireTracer logs SIP READ/WRITE lines at debug level via logrus. Installed
// globally on the sipgo package by EnableWireTrace.
type wireTracer struct{}

func (wireTracer) SIPTraceRead(transport, laddr, raddr string, msg []byte) {
	logrus.Debugf("SIP READ [%s] %s <- %s:\n%s", transport, laddr, raddr, string(msg))
}

func (wireTracer) SIPTraceWrite(transport, laddr, raddr string, msg []byte) {
	logrus.Debugf("SIP WRITE [%s] %s -> %s:\n%s", transport, laddr, raddr, string(msg))
}

// EnableWireTrace turns on SIP wire-level tracing. Every SIP message read
// from or written to the network is logged via logrus at debug level. Call
// once at startup when debug logging is requested. Sets package-global state
// on sipgo, so it affects every SIP client in the process.
func EnableWireTrace() {
	sipgosip.SIPDebug = true
	sipgosip.SIPDebugTracer(wireTracer{})
}

// noisyMediaMessages are diago media-layer log records we suppress because
// they fire dozens of times per second on benign protocol mismatches (e.g.
// Linphone offering rtcp-mux which diago doesn't echo, so RTP strays into the
// RTCP socket and fails to unmarshal).
var noisyMediaMessages = []string{
	"RTCP Unmarshal error",
}

type filteredHandler struct {
	inner slog.Handler
}

func (f filteredHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return f.inner.Enabled(ctx, l)
}

func (f filteredHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, m := range noisyMediaMessages {
		if strings.Contains(r.Message, m) {
			return nil
		}
	}
	return f.inner.Handle(ctx, r)
}

func (f filteredHandler) WithAttrs(as []slog.Attr) slog.Handler {
	return filteredHandler{inner: f.inner.WithAttrs(as)}
}

func (f filteredHandler) WithGroup(name string) slog.Handler {
	return filteredHandler{inner: f.inner.WithGroup(name)}
}

func init() {
	base := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})
	media.SetDefaultLogger(slog.New(filteredHandler{inner: base}))
}
