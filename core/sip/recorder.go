package sip

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"

	bpaudio "github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/callctl"
	diagoaudio "github.com/emiago/diago/audio"
	"github.com/emiago/diago/media"
	log "github.com/sirupsen/logrus"
)

// inboundRingFrames controls how much inbound jitter we'll absorb before
// dropping the oldest samples. 4 frames ≈ 80 ms of headroom, which is
// generous given both directions nominally tick at the same 20 ms cadence.
const inboundRingFrames = 4

// recorder taps the inbound and outbound RTP streams and writes a stereo
// 8 kHz 16-bit WAV file. Caller voice goes on the left channel, IVR audio
// on the right.
//
// Hot-path design:
//   - When recording is *inactive*, FeedInbound / FeedOutbound do exactly
//     one atomic.Bool load and return. No mutex, no allocations.
//   - When recording is *active*, all per-frame state (decoded inbound ring,
//     left scratch buffer, stereo interleave buffer, decoder internal buf)
//     is preallocated on the struct so the hot path makes zero allocations.
//   - The internal mutex is only acquired when active, and hold time is the
//     duration of a few hundred byte writes (microseconds).
//
// Lifecycle:
//   - Constructed once per call (in NewRTPAudioSink). Inactive by default.
//   - Shared by reference with SIPPhone (FeedInbound) and OutputStream
//     (FeedOutbound). Both Feed methods are no-ops while inactive.
//   - Start(path) activates and opens the WAV file.
//   - Stop() finalizes header sizes and closes the file. Returns
//     ErrNotRecording if called when inactive.
type recorder struct {
	codec media.Codec

	// activeFlag is the fast-path guard read without the mutex by
	// FeedInbound / FeedOutbound. Start/Stop set it under the mutex
	// (set true after all state is ready; set false before tearing down).
	activeFlag atomic.Bool

	mu    sync.Mutex // guards everything below
	file  *os.File
	wav   *stereoWavWriter
	dec   *diagoaudio.PCMDecoderWriter // decodes inbound PCMU/PCMA → PCM16 directly into `inbound`
	inbound ringBuffer

	// Per-frame scratch, preallocated as fixed-size arrays so they live with
	// the recorder for the call's lifetime — no make() in the hot path.
	leftBuf   [bpaudio.FrameBytes]byte
	stereoBuf [bpaudio.FrameBytes * 2]byte
}

func newRecorder(codec media.Codec) *recorder {
	return &recorder{codec: codec}
}

// Start activates the recorder and opens the WAV file at path. The codec
// determines the inbound decoder; output is always 8 kHz 16-bit stereo PCM.
func (r *recorder) Start(path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.activeFlag.Load() {
		return callctl.ErrAlreadyRecording
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("recorder: create %s: %w", path, err)
	}

	wav, err := newStereoWavWriter(f, int(r.codec.SampleRate))
	if err != nil {
		f.Close()
		os.Remove(path)
		return err
	}

	// Reset the ring and wire the decoder to write decoded PCM16 directly
	// into it. PCMDecoderWriter has a one-time internal allocation on its
	// first Write call; subsequent writes reuse that buffer.
	r.inbound.Reset()
	dec := &diagoaudio.PCMDecoderWriter{}
	if err := dec.Init(r.codec, &r.inbound); err != nil {
		wav.Close()
		f.Close()
		os.Remove(path)
		return fmt.Errorf("recorder: decoder init: %w", err)
	}

	r.file = f
	r.wav = wav
	r.dec = dec
	r.activeFlag.Store(true)

	log.WithFields(log.Fields{"path": path, "codec": r.codec.Name}).Info("Recording started")
	return nil
}

// Stop finalizes the WAV and closes the file. Returns ErrNotRecording if
// the recorder is not currently active.
func (r *recorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.activeFlag.Load() {
		return callctl.ErrNotRecording
	}
	// Flip the flag BEFORE tearing down so any in-flight FeedInbound /
	// FeedOutbound that snuck past the fast path will double-check after
	// the mutex and bail out cleanly.
	r.activeFlag.Store(false)

	err := r.wav.Close()
	if cerr := r.file.Close(); err == nil {
		err = cerr
	}
	path := r.file.Name()
	r.file = nil
	r.wav = nil
	r.dec = nil
	r.inbound.Reset()

	log.WithField("path", path).Info("Recording stopped")
	return err
}

// FeedInbound is called from SIPPhone.readDTMFLoop with raw RTP payload
// bytes (codec-encoded — PCMU/PCMA). The decoder writes decoded PCM16
// directly into our ring buffer; no per-frame allocation. No-op when
// inactive.
func (r *recorder) FeedInbound(rtpPayload []byte) {
	if r == nil || !r.activeFlag.Load() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.activeFlag.Load() {
		return
	}
	if _, err := r.dec.Write(rtpPayload); err != nil {
		log.WithError(err).Debug("recorder: inbound decode failed")
	}
}

// FeedOutbound is called from OutputStream.run with one PCM16 20 ms frame
// (320 bytes). Drives the WAV write: pops 320 bytes from the inbound ring
// (zero-padding on short read), interleaves with the outbound frame into
// the preallocated stereo buffer, and writes one stereo block to the file.
//
// Anchoring the cadence to outbound gives us a stable 20 ms clock — diago's
// RTP ticker paces enc.Write — and turning every frame into a fixed-size
// write keeps the WAV duration aligned to wall-clock.
//
// No-op when inactive. Zero allocations when active.
func (r *recorder) FeedOutbound(pcm16Out []byte) {
	if r == nil || !r.activeFlag.Load() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.activeFlag.Load() {
		return
	}

	n := len(pcm16Out)
	if n > len(r.leftBuf) {
		// Defensive: a frame larger than our scratch shouldn't happen
		// (FrameBytes is the canonical size). Drop the frame rather than
		// allocate.
		return
	}

	// Pop n bytes of inbound into leftBuf. Short reads are zero-padded
	// (silence) — happens at call start before the first inbound packet,
	// or if the inbound ring is briefly empty.
	got := r.inbound.Read(r.leftBuf[:n])
	for i := got; i < n; i++ {
		r.leftBuf[i] = 0
	}

	// Interleave left + right into stereoBuf in place. Each loop iteration
	// handles one 16-bit sample per channel = 4 output bytes.
	for i := 0; i+1 < n; i += 2 {
		r.stereoBuf[i*2] = r.leftBuf[i]
		r.stereoBuf[i*2+1] = r.leftBuf[i+1]
		r.stereoBuf[i*2+2] = pcm16Out[i]
		r.stereoBuf[i*2+3] = pcm16Out[i+1]
	}

	if _, err := r.wav.Write(r.stereoBuf[:n*2]); err != nil {
		log.WithError(err).Debug("recorder: stereo write failed")
	}
}

// IsActive reports whether the recorder is currently writing.
func (r *recorder) IsActive() bool {
	if r == nil {
		return false
	}
	return r.activeFlag.Load()
}

// ringBuffer is a fixed-size byte ring with oldest-drop overflow. It
// implements io.Writer (used by the inbound PCM decoder) and exposes a
// Read for the outbound interleave consumer.
//
// The buffer holds at most inboundRingFrames * FrameBytes bytes; once full,
// further writes overwrite the oldest data. This bounds the worst-case
// memory and keeps the recorder cost constant regardless of call length.
type ringBuffer struct {
	buf [inboundRingFrames * bpaudio.FrameBytes]byte
	r   int // next read index
	w   int // next write index
	n   int // number of valid bytes, 0..cap
}

// cap is the maximum number of bytes the ring holds. Computed as a constant
// so the compiler can fold it into bounds checks.
func (rb *ringBuffer) capBytes() int { return len(rb.buf) }

// Write copies p into the ring, wrapping at capacity. If p is larger than
// the ring's free space, the oldest data is overwritten. Always returns
// (len(p), nil) — the caller (PCM decoder) cannot benefit from a short
// write.
func (rb *ringBuffer) Write(p []byte) (int, error) {
	pn := len(p)
	cap := rb.capBytes()

	// If the incoming chunk is larger than the entire ring, keep only the
	// most recent cap bytes. Skip the prefix we'd overwrite anyway.
	if pn > cap {
		p = p[pn-cap:]
		pn = cap
		// Reset to a clean "ring full of newest" state.
		copy(rb.buf[:], p)
		rb.r = 0
		rb.w = 0
		rb.n = cap
		return len(p), nil
	}

	// First write segment: from w to end of buf.
	end := cap - rb.w
	if end > pn {
		end = pn
	}
	copy(rb.buf[rb.w:], p[:end])
	rb.w = (rb.w + end) % cap

	// Second segment: wrap to start.
	if rest := pn - end; rest > 0 {
		copy(rb.buf[:rest], p[end:])
		rb.w = rest
	}

	// Advance occupied count; if we overran, advance read index too
	// (oldest-drop policy).
	rb.n += pn
	if rb.n > cap {
		over := rb.n - cap
		rb.r = (rb.r + over) % cap
		rb.n = cap
	}
	return len(p), nil
}

// Read pops up to len(p) bytes from the ring into p, in FIFO order.
// Returns the number actually read; the caller is responsible for
// handling short reads (the recorder zero-pads).
func (rb *ringBuffer) Read(p []byte) int {
	want := len(p)
	if want > rb.n {
		want = rb.n
	}
	if want == 0 {
		return 0
	}
	cap := rb.capBytes()

	end := cap - rb.r
	if end > want {
		end = want
	}
	copy(p[:end], rb.buf[rb.r:rb.r+end])
	rb.r = (rb.r + end) % cap

	if rest := want - end; rest > 0 {
		copy(p[end:end+rest], rb.buf[:rest])
		rb.r = rest
	}
	rb.n -= want
	return want
}

// Reset clears the ring. Doesn't zero the backing array (no need — Read
// is bounded by the byte count).
func (rb *ringBuffer) Reset() {
	rb.r = 0
	rb.w = 0
	rb.n = 0
}

// stereoWavWriter writes a minimal RIFF/WAVE 16-bit stereo PCM file.
// Header is written at construction with placeholder sizes; Close patches
// the RIFF and data chunk sizes once the final byte count is known.
type stereoWavWriter struct {
	w          io.WriteSeeker
	sampleRate int
	dataBytes  uint32
}

func newStereoWavWriter(w io.WriteSeeker, sampleRate int) (*stereoWavWriter, error) {
	sw := &stereoWavWriter{w: w, sampleRate: sampleRate}
	if err := sw.writeHeader(); err != nil {
		return nil, err
	}
	return sw, nil
}

func (sw *stereoWavWriter) writeHeader() error {
	const (
		numChannels   = 2
		bitsPerSample = 16
	)
	byteRate := sw.sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8

	var hdr [44]byte
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], 36)
	copy(hdr[8:12], "WAVE")
	copy(hdr[12:16], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16)
	binary.LittleEndian.PutUint16(hdr[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(hdr[22:24], numChannels)
	binary.LittleEndian.PutUint32(hdr[24:28], uint32(sw.sampleRate))
	binary.LittleEndian.PutUint32(hdr[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(hdr[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(hdr[34:36], bitsPerSample)
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], 0)

	_, err := sw.w.Write(hdr[:])
	return err
}

func (sw *stereoWavWriter) Write(p []byte) (int, error) {
	n, err := sw.w.Write(p)
	sw.dataBytes += uint32(n)
	return n, err
}

func (sw *stereoWavWriter) Close() error {
	if _, err := sw.w.Seek(4, io.SeekStart); err != nil {
		return err
	}
	if err := binary.Write(sw.w, binary.LittleEndian, 36+sw.dataBytes); err != nil {
		return err
	}
	if _, err := sw.w.Seek(40, io.SeekStart); err != nil {
		return err
	}
	if err := binary.Write(sw.w, binary.LittleEndian, sw.dataBytes); err != nil {
		return err
	}
	return nil
}
