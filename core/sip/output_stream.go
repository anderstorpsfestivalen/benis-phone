package sip

import (
	"errors"
	"io"
	"sync/atomic"

	bpaudio "github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/emiago/diago"
	diagoaudio "github.com/emiago/diago/audio"
	"github.com/emiago/diago/media"
	log "github.com/sirupsen/logrus"
)

// sourceJob is one play request handed to the OutputStream. The done channel
// receives nil on natural EOF, ErrInterrupted on preemption/shutdown, or the
// underlying write error if RTP fails.
type sourceJob struct {
	src  bpaudio.Source
	done chan error
}

// OutputStream owns one diago AudioWriter (wrapped by a PCMEncoderWriter) and
// a goroutine that pulls 20 ms PCM frames from the current source and writes
// them out as the negotiated codec. There is exactly one OutputStream per SIP
// call.
type OutputStream struct {
	enc   io.Writer    // *diagoaudio.PCMEncoderWriter
	codec media.Codec

	queue    chan sourceJob
	clear    chan struct{}
	pause    chan struct{}
	resume   chan struct{}
	shutdown chan struct{}
	doneAck  chan struct{}

	playing atomic.Bool
}

// NewOutputStream creates and starts the per-call output goroutine. It pulls
// the AudioWriter and the negotiated codec from the dialog in one call.
func NewOutputStream(dialog *diago.DialogServerSession) (*OutputStream, error) {
	var mprops diago.MediaProps
	writer, err := dialog.AudioWriter(diago.WithAudioWriterMediaProps(&mprops))
	if err != nil {
		return nil, err
	}

	enc := &diagoaudio.PCMEncoderWriter{}
	if err := enc.Init(mprops.Codec, writer); err != nil {
		return nil, err
	}

	os := &OutputStream{
		enc:      enc,
		codec:    mprops.Codec,
		queue:    make(chan sourceJob, 8),
		clear:    make(chan struct{}, 1),
		pause:    make(chan struct{}, 1),
		resume:   make(chan struct{}, 1),
		shutdown: make(chan struct{}),
		doneAck:  make(chan struct{}),
	}

	log.WithFields(log.Fields{
		"codec_name":        mprops.Codec.Name,
		"codec_sample_rate": mprops.Codec.SampleRate,
		"codec_channels":    mprops.Codec.NumChannels,
		"codec_payload":     mprops.Codec.PayloadType,
		"local_addr":        mprops.Laddr,
		"remote_addr":       mprops.Raddr,
	}).Info("OutputStream: negotiated codec")

	go os.run()
	return os, nil
}

// Codec returns the negotiated media codec.
func (os *OutputStream) Codec() media.Codec { return os.codec }

// Submit enqueues a source for playback. Returns a channel that receives the
// terminal result of this specific job. Blocks only if the queue is full
// (rare; preemption uses Clear+Submit, not queueing).
func (os *OutputStream) Submit(src bpaudio.Source) <-chan error {
	done := make(chan error, 1)
	select {
	case os.queue <- sourceJob{src: src, done: done}:
	case <-os.doneAck:
		done <- errors.New("audio: output stream closed")
	}
	return done
}

// Clear preempts the current source and drops any queued sources. Non-blocking.
func (os *OutputStream) Clear() {
	select {
	case os.clear <- struct{}{}:
	default:
	}
}

// Pause stops pulling frames from the current source but keeps it as-current.
// Resume continues from where it left off. Non-blocking.
func (os *OutputStream) Pause() {
	select {
	case os.pause <- struct{}{}:
	default:
	}
}

func (os *OutputStream) Resume() {
	select {
	case os.resume <- struct{}{}:
	default:
	}
}

// IsPlaying reports whether a source is currently emitting frames.
func (os *OutputStream) IsPlaying() bool { return os.playing.Load() }

// Close stops the goroutine, drains the queue (signaling ErrInterrupted to
// blocked callers), and waits for the goroutine to exit.
func (os *OutputStream) Close() {
	select {
	case <-os.doneAck:
		return
	default:
	}
	close(os.shutdown)
	<-os.doneAck
}

// run is the per-call output goroutine.
func (os *OutputStream) run() {
	defer close(os.doneAck)

	buf := make([]byte, bpaudio.FrameBytes)
	var cur bpaudio.Source
	var curDone chan error
	paused := false

	finishCurrent := func(err error) {
		if cur != nil {
			cur.Close()
			cur = nil
		}
		if curDone != nil {
			curDone <- err
			curDone = nil
		}
		os.playing.Store(false)
	}

	drainQueue := func(err error) {
		for {
			select {
			case job := <-os.queue:
				job.src.Close()
				job.done <- err
			default:
				return
			}
		}
	}

	for {
		// Idle: no source. Block waiting for queue or signal.
		if cur == nil {
			select {
			case <-os.shutdown:
				drainQueue(bpaudio.ErrInterrupted)
				return
			case <-os.clear:
				drainQueue(bpaudio.ErrInterrupted)
			case <-os.pause:
				paused = true
			case <-os.resume:
				paused = false
			case job := <-os.queue:
				cur = job.src
				curDone = job.done
				os.playing.Store(true)
			}
			continue
		}

		// Paused with a source held. Block waiting for resume/clear/shutdown.
		if paused {
			select {
			case <-os.shutdown:
				finishCurrent(bpaudio.ErrInterrupted)
				drainQueue(bpaudio.ErrInterrupted)
				return
			case <-os.clear:
				finishCurrent(bpaudio.ErrInterrupted)
				drainQueue(bpaudio.ErrInterrupted)
			case <-os.resume:
				paused = false
			}
			continue
		}

		// Active: poll signals non-blocking, then pull+write one frame.
		select {
		case <-os.shutdown:
			finishCurrent(bpaudio.ErrInterrupted)
			drainQueue(bpaudio.ErrInterrupted)
			return
		case <-os.clear:
			finishCurrent(bpaudio.ErrInterrupted)
			drainQueue(bpaudio.ErrInterrupted)
			continue
		case <-os.pause:
			paused = true
			continue
		default:
		}

		n, err := cur.NextFrame(buf)
		if n > 0 {
			if _, werr := os.enc.Write(buf[:n]); werr != nil {
				log.WithError(werr).Debug("OutputStream write error")
				finishCurrent(werr)
				continue
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.WithError(err).Debug("OutputStream source error")
			}
			if errors.Is(err, io.EOF) {
				finishCurrent(nil)
			} else {
				finishCurrent(err)
			}
		}
	}
}
