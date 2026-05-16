package sip

import (
	"errors"
	"fmt"
	"os"
	"sync"

	bpaudio "github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/emiago/diago"
	"github.com/faiface/beep"
	log "github.com/sirupsen/logrus"
)

// RTPAudioSink implements audio.AudioSink on top of a per-call OutputStream.
type RTPAudioSink struct {
	dialog *diago.DialogServerSession
	os     *OutputStream
}

// Verify RTPAudioSink implements AudioSink
var _ bpaudio.AudioSink = (*RTPAudioSink)(nil)

// NewRTPAudioSink creates a new RTP audio sink for a SIP session.
func NewRTPAudioSink(dialog *diago.DialogServerSession) (*RTPAudioSink, error) {
	os, err := NewOutputStream(dialog)
	if err != nil {
		return nil, fmt.Errorf("create output stream: %w", err)
	}

	return &RTPAudioSink{
		dialog: dialog,
		os:     os,
	}, nil
}

// PlayFromFile plays a local audio file. Blocks until the source ends, is
// preempted, or the call tears down. ErrInterrupted is swallowed (returned as
// nil) so explicit preemption from Clear()/Close() isn't surfaced as an error.
func (r *RTPAudioSink) PlayFromFile(filename string) error {
	src, err := bpaudio.NewFileSource(filename)
	if err != nil {
		return err
	}
	return r.playAndWait(src)
}

// PlayFromStream plays MP3 audio held in memory (Polly TTS output).
func (r *RTPAudioSink) PlayFromStream(data []byte) error {
	src, err := bpaudio.NewMP3StreamSource(data)
	if err != nil {
		return err
	}
	return r.playAndWait(src)
}

// ExternalPlayback submits an externally owned beep streamer to the output
// queue and returns once the source has been queued in the OutputStream's
// ctrl channel — it does NOT wait for the audio to finish.
//
// This is critical for queue.go's pause/resume pattern: the queue runs
// startBackground (this method) and then pausebg (Clear) on the same
// goroutine. With `Submit` being synchronous w.r.t. the ctrl channel, any
// subsequent Clear is guaranteed to land after this Submit, so the bg
// source can actually be preempted. Previously the caller spawned a
// goroutine and the Submit could race with the next Clear, causing bg
// music to play unpaused while the position TTS got queued behind it.
//
// The completion of the source is observed in a goroutine; errors other
// than ErrInterrupted are logged.
func (r *RTPAudioSink) ExternalPlayback(stream beep.StreamSeekCloser, format beep.Format) {
	src := bpaudio.NewBeepSource(stream, format)
	done := r.os.Submit(src)
	go func() {
		err := <-done
		if err != nil && !errors.Is(err, bpaudio.ErrInterrupted) {
			log.WithError(err).Debug("ExternalPlayback ended")
		}
	}()
}

// Clear preempts any currently playing source and drops queued sources.
// Returns immediately; audio stops within one frame (~20 ms).
func (r *RTPAudioSink) Clear() {
	r.os.Clear()
}

// IsPlaying returns true if a source is currently being read.
func (r *RTPAudioSink) IsPlaying() bool {
	return r.os.IsPlaying()
}

// Close shuts down the OutputStream goroutine. Should be called once per call,
// during cleanup, before dialog.Hangup.
func (r *RTPAudioSink) Close() {
	r.os.Close()
}

func (r *RTPAudioSink) playAndWait(src bpaudio.Source) error {
	done := r.os.Submit(src)
	err := <-done
	if errors.Is(err, bpaudio.ErrInterrupted) {
		return nil
	}
	return err
}

// RTPAudioSource implements audio.AudioSource for SIP RTP recording. Recording
// is currently disabled (the diago stereo recorder conflicts with the DTMF
// reader on the same RTP stream and needs a different approach). All methods
// are no-ops until that's resolved.
type RTPAudioSource struct {
	dialog      *diago.DialogServerSession
	recFile     *os.File
	isRecording bool
	recordPath  string
	mu          sync.Mutex
}

var _ bpaudio.AudioSource = (*RTPAudioSource)(nil)

func NewRTPAudioSource(dialog *diago.DialogServerSession, recordPath string) *RTPAudioSource {
	return &RTPAudioSource{
		dialog:     dialog,
		recordPath: recordPath,
	}
}

func (r *RTPAudioSource) Record(subfolder string) {
	log.WithField("subfolder", subfolder).Debug("RTP recording is disabled")
}

func (r *RTPAudioSource) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recFile != nil {
		r.recFile.Close()
		r.recFile = nil
	}
	r.isRecording = false
}

func (r *RTPAudioSource) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.isRecording
}

// NullAudioSource is a no-op AudioSource for code paths that don't record.
type NullAudioSource struct{}

var _ bpaudio.AudioSource = (*NullAudioSource)(nil)

func (n *NullAudioSource) Record(subfolder string) {}
func (n *NullAudioSource) Stop()                   {}
func (n *NullAudioSource) IsRecording() bool       { return false }
