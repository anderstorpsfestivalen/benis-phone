package audio

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/vorbis"
	"github.com/faiface/beep/wav"
	log "github.com/sirupsen/logrus"
)

// Transcoder handles conversion of audio files to telephony-compatible formats.
// SIP/RTP typically uses 8kHz mono PCM (ulaw/alaw).
type Transcoder struct {
	cacheDir   string
	targetRate beep.SampleRate
	mu         sync.RWMutex
}

// NewTranscoder creates a new transcoder with the specified cache directory.
func NewTranscoder(cacheDir string) *Transcoder {
	if cacheDir == "" {
		cacheDir = "transcache"
	}
	os.MkdirAll(cacheDir, os.ModePerm)

	return &Transcoder{
		cacheDir:   cacheDir,
		targetRate: beep.SampleRate(8000), // 8kHz for telephony
	}
}

// TranscodeToWav converts any supported audio file to 8kHz mono WAV.
// Returns path to the transcoded file (cached) or error.
func (t *Transcoder) TranscodeToWav(sourcePath string) (string, error) {
	// Generate cache key from source path and modification time
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat source: %w", err)
	}

	cacheKey := fmt.Sprintf("%s_%d", sourcePath, info.ModTime().UnixNano())
	hash := sha1.Sum([]byte(cacheKey))
	cachedPath := filepath.Join(t.cacheDir, fmt.Sprintf("%x.wav", hash))

	// Check if cached version exists
	t.mu.RLock()
	if _, err := os.Stat(cachedPath); err == nil {
		t.mu.RUnlock()
		log.WithField("cached", cachedPath).Debug("Using cached transcoded file")
		return cachedPath, nil
	}
	t.mu.RUnlock()

	// Transcode the file
	t.mu.Lock()
	defer t.mu.Unlock()

	// Double-check after acquiring write lock
	if _, err := os.Stat(cachedPath); err == nil {
		return cachedPath, nil
	}

	log.WithFields(log.Fields{
		"source": sourcePath,
		"target": cachedPath,
	}).Info("Transcoding audio file")

	if err := t.transcode(sourcePath, cachedPath); err != nil {
		return "", err
	}

	return cachedPath, nil
}

// TranscodeMP3ToWav converts MP3 data to 8kHz mono WAV data.
func (t *Transcoder) TranscodeMP3ToWav(mp3Data []byte) ([]byte, error) {
	reader := bytes.NewReader(mp3Data)
	streamer, format, err := mp3.Decode(io.NopCloser(reader))
	if err != nil {
		return nil, fmt.Errorf("failed to decode MP3: %w", err)
	}
	defer streamer.Close()

	return t.streamToWav(streamer, format)
}

// transcode converts source file to target WAV file
func (t *Transcoder) transcode(sourcePath, targetPath string) error {
	f, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer f.Close()

	var streamer beep.StreamSeekCloser
	var format beep.Format

	ext := filepath.Ext(sourcePath)
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
		return fmt.Errorf("unsupported format: %s", ext)
	}

	if err != nil {
		return fmt.Errorf("failed to decode: %w", err)
	}
	defer streamer.Close()

	// Convert to WAV data
	wavData, err := t.streamToWav(streamer, format)
	if err != nil {
		return err
	}

	// Write to target file
	return os.WriteFile(targetPath, wavData, 0644)
}

// streamToWav converts a beep stream to 8kHz mono WAV data
func (t *Transcoder) streamToWav(streamer beep.Streamer, format beep.Format) ([]byte, error) {
	// Resample to target rate
	resampled := beep.Resample(4, format.SampleRate, t.targetRate, streamer)

	// Collect all samples
	var samples []int16
	buf := make([][2]float64, 512)

	for {
		n, ok := resampled.Stream(buf)
		if n == 0 {
			break
		}

		for i := 0; i < n; i++ {
			// Mix stereo to mono
			mono := (buf[i][0] + buf[i][1]) / 2.0

			// Clamp to [-1, 1]
			if mono > 1.0 {
				mono = 1.0
			} else if mono < -1.0 {
				mono = -1.0
			}

			// Convert to int16
			samples = append(samples, int16(mono*32767))
		}

		if !ok {
			break
		}
	}

	// Create WAV file in memory
	return createWav(samples, int(t.targetRate))
}

// createWav creates a WAV file from PCM samples
func createWav(samples []int16, sampleRate int) ([]byte, error) {
	var buf bytes.Buffer

	numChannels := 1
	bitsPerSample := 16
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataSize := len(samples) * 2

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+dataSize)) // File size - 8
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))               // Chunk size
	binary.Write(&buf, binary.LittleEndian, uint16(1))                // Audio format (PCM)
	binary.Write(&buf, binary.LittleEndian, uint16(numChannels))      // Channels
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))       // Sample rate
	binary.Write(&buf, binary.LittleEndian, uint32(byteRate))         // Byte rate
	binary.Write(&buf, binary.LittleEndian, uint16(blockAlign))       // Block align
	binary.Write(&buf, binary.LittleEndian, uint16(bitsPerSample))    // Bits per sample

	// data chunk
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(dataSize))

	// Write samples
	for _, s := range samples {
		binary.Write(&buf, binary.LittleEndian, s)
	}

	return buf.Bytes(), nil
}
