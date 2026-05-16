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

// opKind enumerates the control operations the OutputStream goroutine handles.
type opKind int

const (
	opSubmit opKind = iota
	opClear
	opPause
	opResume
	opShutdown
)

// op is one control operation queued on the OutputStream's single FIFO ctrl
// channel. Using one channel for everything guarantees that a Clear sent
// before a Submit is processed first — separate channels would let Go's
// random select reorder them.
type op struct {
	kind opKind
	job  *sourceJob // populated for opSubmit
}

// OutputStream owns one diago AudioWriter wrapped by a PCMEncoderWriter and
// the goroutine that paces 20 ms PCM frames onto the wire. There is exactly
// one OutputStream per SIP call.
//
// Design notes:
//   - All control operations (Submit, Clear, Pause, Resume, Shutdown) go
//     through a single buffered `ctrl` channel so the goroutine sees them
//     in submission order. Previously these were separate channels and Go's
//     select would randomly reorder a "Clear then Submit" pair, which made
//     a freshly-submitted source occasionally get drained by the Clear.
//   - Whenever no source is active (idle, paused, or starved between frames
//     of a slow source) the goroutine writes a silence frame so the RTP
//     stream stays continuous. This avoids receivers' jitter buffers
//     dropping the first packets of a new playback after an idle gap, which
//     was the audible "skip the beginning of the sound" symptom.
type OutputStream struct {
	enc   io.Writer // *diagoaudio.PCMEncoderWriter
	codec media.Codec

	ctrl    chan op       // FIFO of all control ops; buffered
	doneAck chan struct{} // closed when goroutine exits

	playing atomic.Bool
}

// NewOutputStream starts the per-call output goroutine.
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
		enc:     enc,
		codec:   mprops.Codec,
		ctrl:    make(chan op, 64),
		doneAck: make(chan struct{}),
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
// terminal result of this specific job: nil on natural EOF, ErrInterrupted on
// preemption, or the underlying error if RTP/encoder writes fail.
//
// Submit blocks only if the ctrl channel is full (64 ops pending — very
// unlikely under normal IVR load); it then waits for either room or for the
// OutputStream to be closed.
func (os *OutputStream) Submit(src bpaudio.Source) <-chan error {
	done := make(chan error, 1)
	select {
	case os.ctrl <- op{kind: opSubmit, job: &sourceJob{src: src, done: done}}:
	case <-os.doneAck:
		done <- errors.New("audio: output stream closed")
	}
	return done
}

// Clear preempts the current source and drops any pending sources.
// Non-blocking: if the ctrl channel is momentarily full, the Clear is dropped
// (caller's next operation will re-clear if needed). Audio stops within ~20 ms.
func (os *OutputStream) Clear() {
	select {
	case os.ctrl <- op{kind: opClear}:
	default:
	}
}

// Pause stops pulling frames from the current source but keeps it as-current.
// Resume continues from where it left off. Non-blocking.
func (os *OutputStream) Pause() {
	select {
	case os.ctrl <- op{kind: opPause}:
	default:
	}
}

func (os *OutputStream) Resume() {
	select {
	case os.ctrl <- op{kind: opResume}:
	default:
	}
}

// IsPlaying reports whether a source is currently being read.
func (os *OutputStream) IsPlaying() bool { return os.playing.Load() }

// Close shuts down the goroutine, signaling ErrInterrupted to any blocked
// callers, and waits for the goroutine to exit.
func (os *OutputStream) Close() {
	select {
	case <-os.doneAck:
		return
	default:
	}
	// Use blocking send so shutdown can't be lost even if ctrl is briefly full.
	select {
	case os.ctrl <- op{kind: opShutdown}:
	case <-os.doneAck:
	}
	<-os.doneAck
}

// run is the per-call output goroutine.
func (os *OutputStream) run() {
	defer close(os.doneAck)

	silence := make([]byte, bpaudio.FrameBytes) // s16le zeros
	buf := make([]byte, bpaudio.FrameBytes)

	var cur bpaudio.Source
	var curDone chan error
	var pending []sourceJob
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

	dropPending := func(err error) {
		for _, j := range pending {
			j.src.Close()
			j.done <- err
		}
		pending = pending[:0]
	}

	// handle processes one control op. Returns true if the goroutine should exit.
	handle := func(o op) bool {
		switch o.kind {
		case opShutdown:
			finishCurrent(bpaudio.ErrInterrupted)
			dropPending(bpaudio.ErrInterrupted)
			return true
		case opClear:
			finishCurrent(bpaudio.ErrInterrupted)
			dropPending(bpaudio.ErrInterrupted)
		case opPause:
			paused = true
		case opResume:
			paused = false
		case opSubmit:
			if cur == nil {
				cur = o.job.src
				curDone = o.job.done
				os.playing.Store(true)
			} else {
				pending = append(pending, *o.job)
			}
		}
		return false
	}

	// drainOps processes every queued op non-blocking. Returns true on shutdown.
	drainOps := func() bool {
		for {
			select {
			case o := <-os.ctrl:
				if handle(o) {
					return true
				}
			default:
				return false
			}
		}
	}

	// promoteNext picks the next pending source, if any, after EOF.
	promoteNext := func() {
		if len(pending) == 0 {
			return
		}
		cur = pending[0].src
		curDone = pending[0].done
		pending = pending[1:]
		os.playing.Store(true)
	}

	for {
		if drainOps() {
			return
		}

		// Active path: pull a real frame from the current source.
		if cur != nil && !paused {
			n, err := cur.NextFrame(buf)
			if n > 0 {
				if _, werr := os.enc.Write(buf[:n]); werr != nil {
					log.WithError(werr).Debug("OutputStream write error")
					finishCurrent(werr)
					continue
				}
			} else if err == nil {
				// Source produced no data this tick (e.g. mp3 decoder primed).
				// Emit silence to keep RTP cadence; try again next tick.
				if _, werr := os.enc.Write(silence); werr != nil {
					log.WithError(werr).Debug("OutputStream silence write failed")
					finishCurrent(werr)
					continue
				}
			}
			if errors.Is(err, io.EOF) {
				finishCurrent(nil)
				promoteNext() // Seamless transition to next queued source.
				continue
			}
			if err != nil {
				log.WithError(err).Debug("OutputStream source error")
				finishCurrent(err)
				continue
			}
			continue
		}

		// Idle or paused: write silence at the 20 ms cadence diago's writer
		// enforces. This keeps the RTP stream continuous so receivers don't
		// reset their jitter buffers when real audio starts again.
		if _, werr := os.enc.Write(silence); werr != nil {
			log.WithError(werr).Warn("OutputStream silence write failed; exiting")
			finishCurrent(werr)
			dropPending(werr)
			return
		}
	}
}
