package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/faiface/beep"
	"github.com/gen2brain/malgo"
)

// InputDevice describes a single capture device returned by
// EnumerateInputDevices.
type InputDevice struct {
	Name     string
	ID       malgo.DeviceID
	Channels uint32
	Rate     uint32 // native sample rate (Hz)
}

// EnumerateInputDevices returns one entry per OS audio capture device. The
// context is set up and torn down inside the call; cheap to invoke ad-hoc
// (used by the CLI -list-audio-devices flag and the CaptureSource device
// resolver).
func EnumerateInputDevices() ([]InputDevice, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("audio: malgo context: %w", err)
	}
	defer func() {
		_ = ctx.Uninit()
		ctx.Free()
	}()

	infos, err := ctx.Devices(malgo.Capture)
	if err != nil {
		return nil, fmt.Errorf("audio: enumerate capture devices: %w", err)
	}

	out := make([]InputDevice, 0, len(infos))
	for _, info := range infos {
		// Pull a full DeviceInfo so we get format/channel data — Devices()
		// returns minimal info on some backends.
		full, err := ctx.DeviceInfo(malgo.Capture, info.ID, malgo.Shared)
		if err != nil {
			out = append(out, InputDevice{Name: info.Name(), ID: info.ID})
			continue
		}
		dev := InputDevice{Name: full.Name(), ID: full.ID}
		if len(full.Formats) > 0 {
			dev.Channels = full.Formats[0].Channels
			dev.Rate = full.Formats[0].SampleRate
		}
		out = append(out, dev)
	}
	return out, nil
}

// resolveDevice picks the first capture device whose name contains `query`
// (case-insensitive). Empty query returns the system default (IsDefault == 1
// if present, otherwise first listed).
func resolveDevice(query string) (InputDevice, error) {
	devs, err := EnumerateInputDevices()
	if err != nil {
		return InputDevice{}, err
	}
	if len(devs) == 0 {
		return InputDevice{}, fmt.Errorf("audio: no capture devices available")
	}
	if query == "" {
		return devs[0], nil
	}
	needle := strings.ToLower(query)
	for _, d := range devs {
		if strings.Contains(strings.ToLower(d.Name), needle) {
			return d, nil
		}
	}
	names := make([]string, len(devs))
	for i, d := range devs {
		names[i] = d.Name
	}
	return InputDevice{}, fmt.Errorf("audio: no capture device matched %q (available: %s)", query, strings.Join(names, ", "))
}

// captureRingFrames bounds how much audio the malgo callback may buffer
// before the pull side (NextFrame) catches up. 50 × 20 ms = 1 s of headroom —
// generous enough to absorb scheduler jitter without growing unbounded if
// playback stalls.
const captureRingFrames = 50

// CaptureSource turns a malgo capture device into a bpaudio.Source. The
// miniaudio callback (runs on its own thread) pushes int16 interleaved
// samples into a ring; NextFrame pulls them, picks one channel, resamples
// to 8 kHz mono via beep.Resample, and writes 20 ms PCM16 frames.
//
// Underruns are silence-padded rather than blocking — the RTP output must
// keep ticking at 20 ms or the codec breaks.
type CaptureSource struct {
	device   *malgo.Device
	ctx      *malgo.AllocatedContext
	channel  int  // 0-indexed channel selected from the native stream
	channels uint32
	rate     uint32

	mu     sync.Mutex
	ring   []int16 // interleaved native-rate samples for the chosen channel only
	rIdx   int     // next read index
	wIdx   int     // next write index
	count  int     // bytes... no, int16 samples currently valid (single-channel)
	closed bool

	// adapter wraps a streamer that drains the ring; it resamples to 8 kHz
	// and converts to mono s16le frames.
	adapter *beepFrameAdapter
}

// NewCaptureSource opens the named device (substring match, case-insensitive)
// and selects the given 0-indexed channel from its native stream. Returns an
// error if the device can't be opened or the channel doesn't exist.
func NewCaptureSource(deviceName string, channel int) (*CaptureSource, error) {
	if channel < 0 {
		return nil, fmt.Errorf("audio: channel must be >= 0 (got %d)", channel)
	}

	dev, err := resolveDevice(deviceName)
	if err != nil {
		return nil, err
	}
	if dev.Channels > 0 && uint32(channel) >= dev.Channels {
		return nil, fmt.Errorf("audio: device %q has %d channel(s); cannot select index %d", dev.Name, dev.Channels, channel)
	}

	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("audio: malgo context: %w", err)
	}

	cs := &CaptureSource{
		ctx:      ctx,
		channel:  channel,
		channels: dev.Channels,
		rate:     dev.Rate,
		// Ring holds single-channel native-rate samples. Sized for the
		// native rate, not 8 kHz.
		ring: make([]int16, captureRingFrames*int(dev.Rate)*int(FrameDuration/1e6)/1000),
	}
	// Guard against zero-sized ring if we couldn't learn the native rate.
	if len(cs.ring) == 0 {
		cs.ring = make([]int16, 48000) // ~1 s @ 48 kHz fallback
	}

	cfg := malgo.DefaultDeviceConfig(malgo.Capture)
	cfg.Capture.Format = malgo.FormatS16
	// Use a real DeviceID pointer so malgo opens the specific device rather
	// than the system default.
	idCopy := dev.ID
	cfg.Capture.DeviceID = idCopy.Pointer()
	// Let malgo pick channels/rate (native). If the enumeration knew them,
	// hint anyway so opening fails loudly on a mismatch.
	if dev.Channels > 0 {
		cfg.Capture.Channels = dev.Channels
	}
	if dev.Rate > 0 {
		cfg.SampleRate = dev.Rate
	}

	device, err := malgo.InitDevice(ctx.Context, cfg, malgo.DeviceCallbacks{
		Data: cs.onAudio,
	})
	if err != nil {
		_ = ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("audio: init capture device %q: %w", dev.Name, err)
	}

	// Reconcile what we actually got back — miniaudio may have negotiated
	// different channels/rate than we asked for.
	cs.channels = device.CaptureChannels()
	cs.rate = device.SampleRate()
	if cs.channels == 0 || cs.rate == 0 {
		device.Uninit()
		_ = ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("audio: device opened with zero channels/rate")
	}
	if uint32(channel) >= cs.channels {
		device.Uninit()
		_ = ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("audio: device %q opened with %d channel(s); cannot select index %d", dev.Name, cs.channels, channel)
	}
	cs.device = device

	if err := device.Start(); err != nil {
		device.Uninit()
		_ = ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("audio: start capture %q: %w", dev.Name, err)
	}

	cs.adapter = newBeepFrameAdapter(captureStreamer{cs: cs}, beep.SampleRate(cs.rate))
	return cs, nil
}

// onAudio is the malgo capture callback. inputSamples holds framecount
// frames of interleaved s16 across cs.channels. We pull cs.channel out and
// push into the ring.
func (cs *CaptureSource) onAudio(_ []byte, inputSamples []byte, framecount uint32) {
	if framecount == 0 {
		return
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.closed {
		return
	}

	stride := int(cs.channels)
	chOff := cs.channel
	cap := len(cs.ring)

	for f := 0; f < int(framecount); f++ {
		idx := (f*stride + chOff) * 2
		if idx+2 > len(inputSamples) {
			break
		}
		s := int16(binary.LittleEndian.Uint16(inputSamples[idx : idx+2]))
		cs.ring[cs.wIdx] = s
		cs.wIdx++
		if cs.wIdx == cap {
			cs.wIdx = 0
		}
		if cs.count < cap {
			cs.count++
		} else {
			// Drop oldest: advance read index in lockstep with writer.
			cs.rIdx++
			if cs.rIdx == cap {
				cs.rIdx = 0
			}
		}
	}
}

// readSamples pops up to len(dst) int16 samples from the ring into dst.
// Short reads (dst longer than what's buffered) return what was available;
// the caller (the streamer) silence-pads the rest.
func (cs *CaptureSource) readSamples(dst []int16) int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	want := len(dst)
	if want > cs.count {
		want = cs.count
	}
	cap := len(cs.ring)
	for i := 0; i < want; i++ {
		dst[i] = cs.ring[cs.rIdx]
		cs.rIdx++
		if cs.rIdx == cap {
			cs.rIdx = 0
		}
	}
	cs.count -= want
	return want
}

func (cs *CaptureSource) NextFrame(out []byte) (int, error) {
	cs.mu.Lock()
	closed := cs.closed
	cs.mu.Unlock()
	if closed {
		return 0, io.EOF
	}
	return cs.adapter.NextFrame(out)
}

func (cs *CaptureSource) Close() error {
	cs.mu.Lock()
	if cs.closed {
		cs.mu.Unlock()
		return nil
	}
	cs.closed = true
	cs.mu.Unlock()

	if cs.device != nil {
		cs.device.Uninit()
	}
	if cs.ctx != nil {
		_ = cs.ctx.Uninit()
		cs.ctx.Free()
	}
	return nil
}

// captureStreamer adapts a CaptureSource to beep.Streamer so we can run it
// through beep.Resample (re-used in newBeepFrameAdapter). The streamer pulls
// single-channel native-rate samples from the ring and presents them as
// stereo float64 (both L and R = same mono sample), which the adapter then
// mixes back to mono and resamples to 8 kHz.
type captureStreamer struct {
	cs *CaptureSource
	// scratch reused across Stream calls
	scratch []int16
}

func (s captureStreamer) Stream(samples [][2]float64) (int, bool) {
	n := len(samples)
	if cap(s.scratch) < n {
		s.scratch = make([]int16, n)
	}
	scratch := s.scratch[:n]
	got := s.cs.readSamples(scratch)
	// Underrun: silence-pad so RTP keeps ticking.
	for i := got; i < n; i++ {
		scratch[i] = 0
	}
	for i := 0; i < n; i++ {
		v := float64(scratch[i]) / 32768.0
		samples[i][0] = v
		samples[i][1] = v
	}
	// Live source — never returns ok=false. Clear() preempts via the
	// OutputStream ctrl channel, which is independent of stream state.
	return n, true
}

func (s captureStreamer) Err() error { return nil }
