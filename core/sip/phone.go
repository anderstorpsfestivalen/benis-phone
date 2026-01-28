package sip

import (
	"github.com/anderstorpsfestivalen/benis-phone/core/phone"
	"github.com/emiago/diago"
	log "github.com/sirupsen/logrus"
)

// SIPPhone implements phone.FlowPhone for SIP calls.
// Each SIP call gets its own SIPPhone instance.
type SIPPhone struct {
	dialog     *diago.DialogServerSession
	keyChannel chan string
	hookChannel chan bool
	dtmfReader *diago.DTMFReader
	stopDTMF   func()
}

// Verify SIPPhone implements FlowPhone
var _ phone.FlowPhone = (*SIPPhone)(nil)

// NewSIPPhone creates a new SIPPhone for a SIP dialog session.
func NewSIPPhone(dialog *diago.DialogServerSession) *SIPPhone {
	return &SIPPhone{
		dialog:      dialog,
		keyChannel:  make(chan string, 100),
		hookChannel: make(chan bool, 10),
	}
}

// Init initializes the SIP phone and starts DTMF listening.
func (p *SIPPhone) Init() error {
	// Create DTMF reader for RFC 4733 DTMF detection
	p.dtmfReader = p.dialog.AudioReaderDTMF()

	// Set up DTMF callback
	p.dtmfReader.OnDTMF(func(dtmf rune) error {
		key := string(dtmf)
		// Convert * and # to match existing system expectations
		if dtmf == '*' {
			key = "10"
		} else if dtmf == '#' {
			key = "11"
		}

		log.WithFields(log.Fields{
			"dtmf": string(dtmf),
			"key":  key,
		}).Debug("DTMF received")

		select {
		case p.keyChannel <- key:
		default:
			log.Warn("Key channel full, dropping DTMF")
		}
		return nil
	})

	// Signal hook lifted (call answered)
	p.hookChannel <- true

	return nil
}

// Close terminates the SIP phone and signals hook down.
func (p *SIPPhone) Close() {
	// Signal hook slammed (call ended)
	select {
	case p.hookChannel <- false:
	default:
	}

	close(p.keyChannel)
	close(p.hookChannel)
}

// State returns true if the call is active.
func (p *SIPPhone) State() bool {
	// Check dialog state
	return p.dialog != nil
}

// GetKeyChannel returns the channel for DTMF key events.
func (p *SIPPhone) GetKeyChannel() chan string {
	return p.keyChannel
}

// GetHookChannel returns the channel for hook state events.
func (p *SIPPhone) GetHookChannel() chan bool {
	return p.hookChannel
}

// Dialog returns the underlying diago dialog session for media access.
func (p *SIPPhone) Dialog() *diago.DialogServerSession {
	return p.dialog
}
