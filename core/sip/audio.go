package sip

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/emiago/diago"
	"github.com/faiface/beep"
	log "github.com/sirupsen/logrus"
)

// RTPAudioSink implements audio.AudioSink for SIP RTP streams.
type RTPAudioSink struct {
	dialog     *diago.DialogServerSession
	playback   diago.AudioPlaybackControl
	transcoder *audio.Transcoder
	isPlaying  bool
	mu         sync.Mutex
}

// Verify RTPAudioSink implements AudioSink
var _ audio.AudioSink = (*RTPAudioSink)(nil)

// Shared transcoder instance for all RTP audio sinks
var sharedTranscoder *audio.Transcoder
var transcoderOnce sync.Once

func getTranscoder() *audio.Transcoder {
	transcoderOnce.Do(func() {
		sharedTranscoder = audio.NewTranscoder("transcache")
	})
	return sharedTranscoder
}

// NewRTPAudioSink creates a new RTP audio sink for a SIP session.
func NewRTPAudioSink(dialog *diago.DialogServerSession) (*RTPAudioSink, error) {
	pb, err := dialog.PlaybackControlCreate()
	if err != nil {
		return nil, err
	}

	// Log the negotiated codec for debugging
	codec := pb.Codec()
	log.WithFields(log.Fields{
		"codec_name":        codec.Name,
		"codec_sample_rate": codec.SampleRate,
		"codec_channels":    codec.NumChannels,
		"codec_payload":     codec.PayloadType,
	}).Info("RTP audio sink created with negotiated codec")

	return &RTPAudioSink{
		dialog:     dialog,
		playback:   pb,
		transcoder: getTranscoder(),
	}, nil
}

// PlayFromFile plays audio from a file to the SIP caller.
// Supports MP3, WAV, FLAC, OGG - non-WAV files are transcoded automatically.
func (r *RTPAudioSink) PlayFromFile(filename string) error {
	r.mu.Lock()
	r.isPlaying = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.isPlaying = false
		r.mu.Unlock()
	}()

	ext := filepath.Ext(filename)
	playPath := filename

	// Transcode non-WAV files to 8kHz WAV for telephony
	if ext != ".wav" {
		log.WithFields(log.Fields{
			"file": filename,
			"ext":  ext,
		}).Debug("Transcoding audio file for RTP")

		transcoded, err := r.transcoder.TranscodeToWav(filename)
		if err != nil {
			log.WithError(err).Error("Failed to transcode audio file")
			return err
		}
		playPath = transcoded
	}

	log.WithField("file", playPath).Debug("Playing audio to RTP")

	_, err := r.playback.PlayFile(playPath)
	return err
}

// PlayFromStream plays audio from a byte slice to the SIP caller.
// The data is expected to be MP3 format (from AWS Polly TTS).
// It will be transcoded to 8kHz WAV for telephony.
func (r *RTPAudioSink) PlayFromStream(data []byte) error {
	r.mu.Lock()
	r.isPlaying = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.isPlaying = false
		r.mu.Unlock()
	}()

	// Polly returns MP3, transcode to WAV for telephony
	log.WithField("mp3_size", len(data)).Debug("Transcoding MP3 stream for RTP")

	wavData, err := r.transcoder.TranscodeMP3ToWav(data)
	if err != nil {
		log.WithError(err).Error("Failed to transcode MP3 stream")
		return err
	}

	// Verify the WAV header is valid
	if len(wavData) < 44 {
		log.WithField("wav_size", len(wavData)).Error("WAV data too small")
		return fmt.Errorf("WAV data too small: %d bytes", len(wavData))
	}

	// Log WAV header info for debugging
	log.WithFields(log.Fields{
		"wav_size":    len(wavData),
		"riff_magic":  string(wavData[0:4]),
		"wave_magic":  string(wavData[8:12]),
		"fmt_chunk":   string(wavData[12:16]),
		"audio_fmt":   int(wavData[20]) | int(wavData[21])<<8,
		"channels":    int(wavData[22]) | int(wavData[23])<<8,
		"sample_rate": int(wavData[24]) | int(wavData[25])<<8 | int(wavData[26])<<16 | int(wavData[27])<<24,
	}).Debug("Transcoded to WAV, playing to RTP")

	// Debug: Check media session state
	msess := r.dialog.MediaSession()
	if msess != nil {
		log.WithFields(log.Fields{
			"local_addr":  msess.Laddr.String(),
			"remote_addr": msess.Raddr.String(),
			"mode":        msess.Mode,
		}).Debug("Media session state")
	} else {
		log.Error("Media session is nil!")
	}

	// Debug: Try writing raw PCM directly to AudioWriter to test RTP path
	writer, werr := r.dialog.AudioWriter()
	if werr != nil {
		log.WithError(werr).Error("Failed to get AudioWriter")
	} else {
		// Write a test tone (silence) directly - use proper 20ms at 8kHz (160 samples * 2 bytes = 320 bytes)
		testBuf := make([]byte, 320)
		n, terr := writer.Write(testBuf)
		log.WithFields(log.Fields{
			"test_written": n,
			"test_error":   terr,
		}).Debug("Direct AudioWriter test")
	}

	// Log codec details for debugging
	codec := r.playback.Codec()
	log.WithFields(log.Fields{
		"playback_codec":       codec.Name,
		"playback_sample_rate": codec.SampleRate,
		"playback_channels":    codec.NumChannels,
		"playback_bit_depth":   r.playback.BitDepth,
	}).Debug("Playback codec configuration")

	// Try using PlayFile directly with a temp file - this bypasses potential bytes.Reader issues
	tmpFile, terr := os.CreateTemp("", "tts-*.wav")
	if terr != nil {
		log.WithError(terr).Error("Failed to create temp file for audio")
		return terr
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, werr := tmpFile.Write(wavData); werr != nil {
		tmpFile.Close()
		log.WithError(werr).Error("Failed to write WAV to temp file")
		return werr
	}
	tmpFile.Close()

	log.WithField("temp_file", tmpPath).Debug("Playing audio via temp file")
	written, err := r.playback.PlayFile(tmpPath)
	if err != nil {
		log.WithError(err).Error("Failed to play audio to RTP")
	} else {
		log.WithFields(log.Fields{
			"bytes_written":  written,
			"reader_len":     len(wavData),
			"writer_present": writer != nil,
		}).Debug("Audio playback completed")
	}

	return err
}

// Clear stops any currently playing audio.
func (r *RTPAudioSink) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.playback.Stop()
	r.isPlaying = false
}

// IsPlaying returns true if audio is currently streaming.
func (r *RTPAudioSink) IsPlaying() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.isPlaying
}

// ExternalPlayback allows external control over playback with beep streams.
// For RTP, this is a simplified implementation that extracts audio and plays.
func (r *RTPAudioSink) ExternalPlayback(stream beep.StreamSeekCloser, format beep.Format) {
	r.mu.Lock()
	r.isPlaying = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.isPlaying = false
		r.mu.Unlock()
	}()

	// For RTP, we need to convert the beep stream to raw PCM
	// and write it to the RTP stream. This is a simplified approach.
	// A more complete implementation would use proper resampling.

	writer, err := r.dialog.AudioWriter()
	if err != nil {
		log.WithError(err).Error("Failed to get audio writer for external playback")
		return
	}

	// Stream audio samples to RTP
	samples := make([][2]float64, 960) // 20ms at 48kHz
	buf := make([]byte, 1920)          // PCM16 mono

	for {
		n, ok := stream.Stream(samples)
		if !ok || n == 0 {
			break
		}

		// Convert float64 stereo to int16 mono PCM
		for i := 0; i < n; i++ {
			// Mix stereo to mono
			mono := (samples[i][0] + samples[i][1]) / 2.0
			// Convert to int16
			s := int16(mono * 32767)
			buf[i*2] = byte(s)
			buf[i*2+1] = byte(s >> 8)
		}

		if _, err := writer.Write(buf[:n*2]); err != nil {
			log.WithError(err).Error("Failed to write to RTP stream")
			break
		}
	}
}

// RTPAudioSource implements audio.AudioSource for SIP RTP recording.
type RTPAudioSource struct {
	dialog      *diago.DialogServerSession
	recording   *diago.AudioStereoRecordingWav
	recFile     *os.File
	isRecording bool
	recordPath  string
	mu          sync.Mutex
}

// Verify RTPAudioSource implements AudioSource
var _ audio.AudioSource = (*RTPAudioSource)(nil)

// NewRTPAudioSource creates a new RTP audio source for recording.
func NewRTPAudioSource(dialog *diago.DialogServerSession, recordPath string) *RTPAudioSource {
	return &RTPAudioSource{
		dialog:     dialog,
		recordPath: recordPath,
	}
}

// Record starts recording the RTP stream to the specified subfolder.
// TEMPORARILY DISABLED to debug DTMF/audio issues
func (r *RTPAudioSource) Record(subfolder string) {
	log.WithField("subfolder", subfolder).Debug("Recording disabled for debugging")
	// Recording disabled to avoid RTP stream conflicts with DTMF reader
	return

	/* Original recording code - disabled for debugging
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isRecording {
		r.stopRecording()
	}

	// Create directory structure
	dir := filepath.Join(r.recordPath, subfolder)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.WithError(err).Error("Failed to create recording directory")
		return
	}

	// Create filename with timestamp
	tm := time.Now()
	filename := tm.Format("2006-01-02_15-04-05") + ".wav"
	fullPath := filepath.Join(dir, filename)

	// Create recording file
	var err error
	r.recFile, err = os.Create(fullPath)
	if err != nil {
		log.WithError(err).Error("Failed to create recording file")
		return
	}

	// Create stereo recording (captures both directions)
	rec, err := r.dialog.AudioStereoRecordingCreate(r.recFile)
	if err != nil {
		log.WithError(err).Error("Failed to create RTP recording")
		r.recFile.Close()
		r.recFile = nil
		return
	}
	r.recording = &rec

	r.isRecording = true

	log.WithField("path", fullPath).Info("Started RTP recording")
	*/
}

// Stop terminates the current recording.
func (r *RTPAudioSource) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopRecording()
}

// stopRecording is the internal implementation (called with lock held)
func (r *RTPAudioSource) stopRecording() {
	if !r.isRecording {
		return
	}

	if r.recording != nil {
		r.recording.Close()
		r.recording = nil
	}

	if r.recFile != nil {
		r.recFile.Close()
		r.recFile = nil
	}

	r.isRecording = false
	log.Info("Stopped RTP recording")
}

// IsRecording returns true if currently recording.
func (r *RTPAudioSource) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.isRecording
}

// NullAudioSource is a no-op AudioSource for when recording is disabled.
type NullAudioSource struct{}

var _ audio.AudioSource = (*NullAudioSource)(nil)

func (n *NullAudioSource) Record(subfolder string) {}
func (n *NullAudioSource) Stop()                   {}
func (n *NullAudioSource) IsRecording() bool       { return false }
