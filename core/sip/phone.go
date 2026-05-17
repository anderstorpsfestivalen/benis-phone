package sip

import (
	"context"

	"github.com/anderstorpsfestivalen/benis-phone/core/phone"
	"github.com/emiago/diago"
	"github.com/emiago/diago/media"
	log "github.com/sirupsen/logrus"
)

// SIPPhone implements phone.FlowPhone for SIP calls.
// Each SIP call gets its own SIPPhone instance.
type SIPPhone struct {
	dialog      *diago.DialogServerSession
	keyChannel  chan string
	hookChannel chan bool
	dtmfReader  *diago.DTMFReader
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{} // Closed when DTMF read loop ends

	// recorder, when non-nil, taps each inbound RTP read for stereo recording.
	// nil in pre-call setup; set by NewRTPAudioSink wiring before Init().
	recorder *recorder
}

// SetRecorder wires a shared recorder into this phone. Must be called before
// Init() starts the DTMF read loop. Safe to leave nil — FeedInbound on a nil
// recorder is a no-op.
func (p *SIPPhone) SetRecorder(r *recorder) {
	p.recorder = r
}

// Verify SIPPhone implements FlowPhone
var _ phone.FlowPhone = (*SIPPhone)(nil)

// NewSIPPhone creates a new SIPPhone for a SIP dialog session.
func NewSIPPhone(dialog *diago.DialogServerSession) *SIPPhone {
	return &SIPPhone{
		dialog:      dialog,
		keyChannel:  make(chan string, 100),
		hookChannel: make(chan bool, 10),
		done:        make(chan struct{}),
	}
}

// Init initializes the SIP phone and starts DTMF listening.
func (p *SIPPhone) Init() error {
	// Create context for DTMF reading goroutine
	p.ctx, p.cancel = context.WithCancel(context.Background())

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

	// Start DTMF reading goroutine - this is required for DTMF detection to work
	go p.readDTMFLoop()

	// Signal hook lifted (call answered)
	p.hookChannel <- true

	return nil
}

// readDTMFLoop continuously reads from the DTMF reader to detect DTMF tones.
// DTMF detection in RFC 4733 requires actively reading the RTP stream.
// This loop also keeps the RTP session alive - when it ends, the call is considered terminated.
func (p *SIPPhone) readDTMFLoop() {
	// Log media session info at start
	msess := p.dialog.MediaSession()
	if msess != nil {
		log.WithFields(log.Fields{
			"local_addr":  msess.Laddr.String(),
			"remote_addr": msess.Raddr.String(),
			"mode":        msess.Mode,
		}).Debug("Starting DTMF read loop with media session")
	} else {
		log.Error("DTMF read loop: Media session is nil!")
	}

	defer close(p.done) // Signal that the loop has ended

	buf := make([]byte, media.RTPBufSize)
	readCount := 0
	for {
		select {
		case <-p.ctx.Done():
			log.WithField("reads", readCount).Debug("DTMF read loop stopped (context canceled)")
			return
		default:
			// Read from DTMF reader - this triggers DTMF detection via OnDTMF callback
			n, err := p.dtmfReader.Read(buf)
			if err != nil {
				// Check if context was canceled
				select {
				case <-p.ctx.Done():
					return
				default:
					log.WithFields(log.Fields{
						"error": err,
						"reads": readCount,
					}).Debug("DTMF read error, call ended")
					return
				}
			}
			readCount++
			if readCount == 1 {
				log.WithField("bytes", n).Debug("First DTMF read - RTP stream active")
			}
			if readCount%100 == 0 {
				log.WithField("reads", readCount).Trace("DTMF read loop active")
			}
			// Tap for recording. No-op when recorder is nil or inactive.
			// Hot path: a short mutex on FeedInbound when active, plain nil
			// check otherwise.
			p.recorder.FeedInbound(buf[:n])
		}
	}
}

// Done returns a channel that is closed when the DTMF read loop ends (call terminated).
func (p *SIPPhone) Done() <-chan struct{} {
	return p.done
}

// Close terminates the SIP phone and signals hook down.
func (p *SIPPhone) Close() {
	// Stop DTMF reading goroutine
	if p.cancel != nil {
		p.cancel()
	}

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
