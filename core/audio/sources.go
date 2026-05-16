package audio

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/vorbis"
	"github.com/faiface/beep/wav"
)

// beepFrameAdapter pulls 20 ms frames of 8 kHz mono s16le from a beep stream.
// Resamples to 8 kHz on the fly. Does not own the underlying streamer.
type beepFrameAdapter struct {
	stream    beep.Streamer
	sampleBuf [][2]float64
}

func newBeepFrameAdapter(s beep.Streamer, srcRate beep.SampleRate) *beepFrameAdapter {
	target := beep.SampleRate(FrameRate)
	resampled := beep.Resample(4, srcRate, target, s)
	return &beepFrameAdapter{
		stream:    resampled,
		sampleBuf: make([][2]float64, FrameSamples),
	}
}

func (a *beepFrameAdapter) NextFrame(out []byte) (int, error) {
	if len(out) < FrameBytes {
		return 0, fmt.Errorf("audio: buffer too small, need %d got %d", FrameBytes, len(out))
	}
	n, ok := a.stream.Stream(a.sampleBuf)
	if n == 0 {
		if !ok {
			return 0, io.EOF
		}
		return 0, nil
	}
	nb := ConvertF64StereoToI16LE(a.sampleBuf[:n], out)
	if !ok {
		return nb, io.EOF
	}
	return nb, nil
}

// FileSource plays a local audio file (mp3/wav/flac/ogg) as 8 kHz mono frames.
type FileSource struct {
	file     io.Closer
	streamer beep.StreamSeekCloser
	adapter  *beepFrameAdapter
}

func NewFileSource(path string) (*FileSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		streamer, format, err = mp3.Decode(f)
	case ".wav":
		streamer, format, err = wav.Decode(f)
	case ".flac":
		streamer, format, err = flac.Decode(f)
	case ".ogg":
		streamer, format, err = vorbis.Decode(f)
	default:
		f.Close()
		return nil, fmt.Errorf("audio: unsupported file format %q", ext)
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("audio: decode %s: %w", path, err)
	}

	return &FileSource{
		file:     f,
		streamer: streamer,
		adapter:  newBeepFrameAdapter(streamer, format.SampleRate),
	}, nil
}

func (s *FileSource) NextFrame(out []byte) (int, error) {
	return s.adapter.NextFrame(out)
}

func (s *FileSource) Close() error {
	if s.streamer != nil {
		s.streamer.Close()
	}
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}

// MP3StreamSource plays MP3 audio held in memory (e.g. Polly TTS bytes).
type MP3StreamSource struct {
	streamer beep.StreamSeekCloser
	adapter  *beepFrameAdapter
}

func NewMP3StreamSource(data []byte) (*MP3StreamSource, error) {
	r := bytes.NewReader(data)
	streamer, format, err := mp3.Decode(io.NopCloser(r))
	if err != nil {
		return nil, fmt.Errorf("audio: decode mp3 stream: %w", err)
	}
	return &MP3StreamSource{
		streamer: streamer,
		adapter:  newBeepFrameAdapter(streamer, format.SampleRate),
	}, nil
}

func (s *MP3StreamSource) NextFrame(out []byte) (int, error) {
	return s.adapter.NextFrame(out)
}

func (s *MP3StreamSource) Close() error {
	if s.streamer != nil {
		return s.streamer.Close()
	}
	return nil
}

// BeepSource wraps an externally owned beep streamer (e.g. the Queue's
// pre-decoded background music). Close() does NOT close the underlying
// streamer — the caller retains ownership so it can Seek and resume.
type BeepSource struct {
	adapter *beepFrameAdapter
}

func NewBeepSource(s beep.StreamSeekCloser, format beep.Format) *BeepSource {
	return &BeepSource{
		adapter: newBeepFrameAdapter(s, format.SampleRate),
	}
}

func (b *BeepSource) NextFrame(out []byte) (int, error) {
	return b.adapter.NextFrame(out)
}

func (b *BeepSource) Close() error { return nil }
