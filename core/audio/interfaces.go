package audio

import "github.com/faiface/beep"

// AudioSink represents an audio output destination.
// Can be implemented by local speaker (Audio) or RTP stream.
type AudioSink interface {
	// PlayFromFile plays audio from a file (MP3, WAV, FLAC, OGG)
	PlayFromFile(filename string) error

	// PlayFromStream plays MP3 audio from a byte slice
	PlayFromStream(data []byte) error

	// Clear stops any currently playing audio
	Clear()

	// IsPlaying returns true if audio is currently streaming
	IsPlaying() bool

	// ExternalPlayback allows external control over playback with beep streams.
	// Used by Queue for background music with pause/resume capability.
	// RTP implementations may provide a no-op or different implementation.
	ExternalPlayback(stream beep.StreamSeekCloser, format beep.Format)
}

// AudioSource represents an audio input source for recording.
// Can be implemented by local microphone (Recorder) or RTP stream.
type AudioSource interface {
	// Record starts recording to the specified subfolder
	Record(subfolder string)

	// Stop terminates the current recording
	Stop()

	// IsRecording returns true if currently recording
	IsRecording() bool
}
