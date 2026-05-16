package audio

import (
	"errors"
	"time"
)

const (
	FrameDuration = 20 * time.Millisecond
	FrameRate     = 8000
	FrameSamples  = 160 // FrameRate * FrameDuration
	FrameBytes    = 320 // FrameSamples * 2 (int16 little-endian)
)

// ErrInterrupted is returned to callers whose source was preempted by Clear()
// or by a shutdown. The RTPAudioSink swallows this for caller convenience.
var ErrInterrupted = errors.New("audio: source interrupted")

// Source yields 20 ms frames of 16-bit PCM little-endian, 8 kHz mono.
// NextFrame fills out (len(out) >= FrameBytes) with one frame, returning the
// number of bytes written. On natural end of stream it returns 0 + io.EOF, or
// a partial final frame followed by io.EOF on the subsequent call. Close
// releases any resources owned by the source.
type Source interface {
	NextFrame(out []byte) (n int, err error)
	Close() error
}
