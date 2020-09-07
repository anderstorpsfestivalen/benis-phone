package audio

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"github.com/faiface/beep/wav"
)

type Audio struct {
	sampleRate beep.SampleRate
	isPlaying  bool
}

func New(samplerate int) *Audio {
	return &Audio{
		sampleRate: beep.SampleRate(samplerate),
	}
}

func (a *Audio) Init() error {
	err := speaker.Init(a.sampleRate, a.sampleRate.N(time.Second/30))
	if err != nil {
		return err
	}
	return nil
}

func (a *Audio) PlayMP3FromStream(data []byte) error {
	a.Clear()

	f := bytes.NewReader(data)
	streamer, format, err := mp3.Decode(ioutil.NopCloser(f))
	if err != nil {
		return err
	}

	defer streamer.Close()

	err = a.playback(streamer, format)
	if err != nil {
		return err
	}

	return nil
}

//PlayFromFile playes a MP3, WAV, FLAC or OGG file from disk.
func (a *Audio) PlayFromFile(filename string) error {
	a.Clear()

	f, err := os.Open(filename)
	if err != nil {
		return err
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format

	switch fileformat := filepath.Ext(filename); fileformat {

	case ".mp3":
		streamer, format, err = a.decodeMP3(f)
		if err != nil {
			return err
		}
	case ".wav":
		streamer, format, err = a.decodeWAV(f)
		if err != nil {
			return err
		}
	case ".flac":
		streamer, format, err = a.decodeFLAC(f)
		if err != nil {
			return err
		}
	case ".ogg":
		streamer, format, err = a.decodeOGG(f)
		if err != nil {
			return err
		}
	}

	defer streamer.Close()

	err = a.playback(streamer, format)
	if err != nil {
		return err
	}

	return nil

}

// Clear stops the currently playing audio
func (a *Audio) Clear() {
	speaker.Clear()
}

/////////////////////////////////////////////////
//// Internal stuff
//////////////////////////////////////////////////

func (a *Audio) playback(stream beep.StreamSeekCloser, format beep.Format) error {
	a.isPlaying = true
	resampled := beep.Resample(4, format.SampleRate, a.sampleRate, stream)

	done := make(chan bool)
	speaker.Play(beep.Seq(resampled, beep.Callback(func() {
		done <- true
	})))

	<-done
	a.isPlaying = false

	return nil
}

/////////////////////////////////////////////////
//// INDIVIDUAL DECODING FUNCTIONS
/////////////////////////////////////////////////

func (a *Audio) decodeMP3(f *os.File) (beep.StreamSeekCloser, beep.Format, error) {
	return mp3.Decode(f)
}

func (a *Audio) decodeWAV(f *os.File) (beep.StreamSeekCloser, beep.Format, error) {
	return wav.Decode(f)
}

func (a *Audio) decodeFLAC(f *os.File) (beep.StreamSeekCloser, beep.Format, error) {
	return flac.Decode(f)
}

func (a *Audio) decodeOGG(f *os.File) (beep.StreamSeekCloser, beep.Format, error) {
	return vorbis.Decode(f)
}
