package sip

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	bpaudio "github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/callctl"
	diagoaudio "github.com/emiago/diago/audio"
	"github.com/emiago/diago/media"
)

// discardSeekWriter is an io.WriteSeeker that accepts every write and tracks
// the current "position" so the WAV header patch logic still works. Used
// in benchmarks where we don't care about the actual bytes.
type discardSeekWriter struct {
	pos int64
}

func (d *discardSeekWriter) Write(p []byte) (int, error) {
	d.pos += int64(len(p))
	return len(p), nil
}

func (d *discardSeekWriter) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		d.pos = offset
	case io.SeekCurrent:
		d.pos += offset
	case io.SeekEnd:
		// Unsupported — we never Seek to End here.
		return 0, errors.New("discardSeekWriter: SeekEnd unsupported")
	}
	return d.pos, nil
}

// newBenchRecorder constructs a recorder wired to a discard writer so we
// measure only the recorder's own allocations, with no real file I/O.
func newBenchRecorder(b testing.TB, active bool) *recorder {
	b.Helper()
	rec := newRecorder(media.CodecAudioUlaw)
	if !active {
		return rec
	}
	wav, err := newStereoWavWriter(&discardSeekWriter{}, int(rec.codec.SampleRate))
	if err != nil {
		b.Fatalf("wav init: %v", err)
	}
	dec := &diagoaudio.PCMDecoderWriter{}
	if err := dec.Init(rec.codec, &rec.inbound); err != nil {
		b.Fatalf("dec init: %v", err)
	}
	rec.mu.Lock()
	rec.wav = wav
	rec.dec = dec
	rec.activeFlag.Store(true)
	rec.mu.Unlock()
	return rec
}

// BenchmarkRecorderHotPath measures the per-frame cost when actively
// recording. Goal: 0 allocs/op after Phase A.
func BenchmarkRecorderHotPath(b *testing.B) {
	rec := newBenchRecorder(b, true)

	// One outbound frame = FrameBytes of PCM16.
	pcm := make([]byte, bpaudio.FrameBytes)
	// One inbound payload = FrameSamples of μ-law (1 byte per sample at 8 kHz).
	payload := make([]byte, bpaudio.FrameSamples)
	for i := range payload {
		payload[i] = 0xFF // arbitrary μ-law byte
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec.FeedInbound(payload)
		rec.FeedOutbound(pcm)
	}
}

// BenchmarkRecorderInactive measures the fast-path overhead when the
// recorder is NOT active — the common case (most calls never record).
// Goal: 0 allocs/op, sub-µs per op.
func BenchmarkRecorderInactive(b *testing.B) {
	rec := newBenchRecorder(b, false)
	pcm := make([]byte, bpaudio.FrameBytes)
	payload := make([]byte, bpaudio.FrameSamples)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec.FeedInbound(payload)
		rec.FeedOutbound(pcm)
	}
}

// --- correctness ---

func TestRingBufferBasicReadWrite(t *testing.T) {
	var rb ringBuffer
	in := []byte{1, 2, 3, 4, 5}
	n, err := rb.Write(in)
	if err != nil || n != len(in) {
		t.Fatalf("Write: n=%d err=%v", n, err)
	}
	out := make([]byte, 5)
	got := rb.Read(out)
	if got != 5 {
		t.Fatalf("Read: got=%d want=5", got)
	}
	if !bytes.Equal(out, in) {
		t.Fatalf("Read mismatch: got=%v want=%v", out, in)
	}
}

func TestRingBufferWrapAround(t *testing.T) {
	var rb ringBuffer
	// Fill to near-cap, drain partially, then write more to force wrap.
	cap := rb.capBytes()
	chunk := make([]byte, cap-10)
	for i := range chunk {
		chunk[i] = byte(i % 256)
	}
	rb.Write(chunk)
	// Drain half so we know there's wrap-room.
	drain := make([]byte, cap/2)
	rb.Read(drain)
	// Now write a chunk that wraps.
	wrapChunk := make([]byte, 50)
	for i := range wrapChunk {
		wrapChunk[i] = byte(0xA0 + i)
	}
	rb.Write(wrapChunk)

	// Read everything, verify the order: rest of original chunk, then wrapChunk.
	remaining := (cap - 10) - cap/2 + 50
	out := make([]byte, remaining)
	got := rb.Read(out)
	if got != remaining {
		t.Fatalf("Read: got=%d want=%d", got, remaining)
	}
	// Last 50 bytes should be wrapChunk in order.
	if !bytes.Equal(out[remaining-50:], wrapChunk) {
		t.Fatalf("wrap region mismatch: got=%v want=%v", out[remaining-50:], wrapChunk)
	}
}

func TestRingBufferOverflowOldestDrop(t *testing.T) {
	var rb ringBuffer
	cap := rb.capBytes()
	// Write 1.5× cap; the oldest 0.5× cap should be dropped.
	chunk := make([]byte, cap+cap/2)
	for i := range chunk {
		chunk[i] = byte(i & 0xFF)
	}
	rb.Write(chunk)
	out := make([]byte, cap)
	got := rb.Read(out)
	if got != cap {
		t.Fatalf("Read got=%d want=%d", got, cap)
	}
	// What remains is the LAST cap bytes of chunk.
	if !bytes.Equal(out, chunk[len(chunk)-cap:]) {
		t.Fatalf("overflow drop wrong: oldest bytes not dropped")
	}
	if rb.Read(out) != 0 {
		t.Fatalf("ring should be empty now")
	}
}

func TestRingBufferShortRead(t *testing.T) {
	var rb ringBuffer
	rb.Write([]byte{1, 2, 3})
	out := make([]byte, 10)
	got := rb.Read(out)
	if got != 3 {
		t.Fatalf("Read: got=%d want=3", got)
	}
	// Caller's responsibility to zero the tail; we just report the count.
}

func TestStereoWavWriterHeader(t *testing.T) {
	buf := &seekableBuffer{}
	wav, err := newStereoWavWriter(buf, 8000)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	// Write 4 stereo samples = 16 bytes.
	wav.Write(make([]byte, 16))
	if err := wav.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	data := buf.Bytes()
	if len(data) != 44+16 {
		t.Fatalf("size: got=%d want=%d", len(data), 44+16)
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		t.Fatalf("magic: %q %q", data[0:4], data[8:12])
	}
	riffSize := binary.LittleEndian.Uint32(data[4:8])
	if riffSize != uint32(36+16) {
		t.Fatalf("RIFF size: got=%d want=%d", riffSize, 36+16)
	}
	dataSize := binary.LittleEndian.Uint32(data[40:44])
	if dataSize != 16 {
		t.Fatalf("data size: got=%d want=16", dataSize)
	}
	channels := binary.LittleEndian.Uint16(data[22:24])
	if channels != 2 {
		t.Fatalf("channels: got=%d want=2", channels)
	}
	sampleRate := binary.LittleEndian.Uint32(data[24:28])
	if sampleRate != 8000 {
		t.Fatalf("sample rate: got=%d want=8000", sampleRate)
	}
}

func TestRecorderStartStopErrors(t *testing.T) {
	rec := newRecorder(media.CodecAudioUlaw)
	if err := rec.Stop(); !errors.Is(err, callctl.ErrNotRecording) {
		t.Fatalf("Stop on inactive: got %v want ErrNotRecording", err)
	}
}

// seekableBuffer is a tiny in-memory io.WriteSeeker for header-correctness tests.
type seekableBuffer struct {
	data []byte
	pos  int64
}

func (b *seekableBuffer) Write(p []byte) (int, error) {
	end := b.pos + int64(len(p))
	if int64(cap(b.data)) < end {
		grow := make([]byte, end)
		copy(grow, b.data)
		b.data = grow
	} else if int64(len(b.data)) < end {
		b.data = b.data[:end]
	}
	copy(b.data[b.pos:], p)
	b.pos = end
	return len(p), nil
}

func (b *seekableBuffer) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		b.pos = offset
	case io.SeekCurrent:
		b.pos += offset
	case io.SeekEnd:
		b.pos = int64(len(b.data)) + offset
	}
	return b.pos, nil
}

func (b *seekableBuffer) Bytes() []byte { return b.data }
