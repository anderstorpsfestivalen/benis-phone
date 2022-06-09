package audio

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"github.com/faiface/beep/wav"
	"gitlab.com/anderstorpsfestivalen/benis-phone/core/broadcast"
)

type Audio struct {
	sampleRate beep.SampleRate
	isPlaying  bool
	cancel     *broadcast.Broadcaster

	ctrl sync.Mutex
}

func New(samplerate int) *Audio {
	return &Audio{
		sampleRate: beep.SampleRate(samplerate),
		cancel:     broadcast.NewBroadcaster(200),
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
	a.ctrl.Lock()
	defer a.ctrl.Unlock()

	a.Clear()

	f := bytes.NewReader(data)
	streamer, format, err := mp3.Decode(ioutil.NopCloser(f))
	if err != nil {
		log.Trace(err)
		return err
	}

	defer streamer.Close()

	a.playback(streamer, format)

	return nil
}

//PlayFromFile playes a MP3, WAV, FLAC or OGG file from disk.
func (a *Audio) PlayFromFile(filename string) error {

	a.Clear()

	f, err := os.Open(filename)
	if err != nil {
		log.Trace(err)
		return err
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format

	switch fileformat := filepath.Ext(filename); fileformat {

	case ".mp3":
		streamer, format, err = mp3.Decode(f)
		if err != nil {
			log.Trace(err)
			return err
		}
	case ".wav":
		streamer, format, err = wav.Decode(f)
		if err != nil {
			log.Trace(err)
			return err
		}
	case ".flac":
		streamer, format, err = flac.Decode(f)
		if err != nil {
			log.Trace(err)
			return err
		}
	case ".ogg":
		streamer, format, err = vorbis.Decode(f)
		if err != nil {
			log.Trace(err)
			return err
		}
	}

	defer streamer.Close()

	a.playback(streamer, format)

	return nil

}

// Clear stops the currently playing audio
func (a *Audio) Clear() {
	speaker.Clear()
	err := a.cancel.Send(true)
	if err != nil {
		log.Trace("Audio channel clear error, unsure why this happens, likely race condition: %v", err.Error())
	}
}

// IsPlaying indicates if there is currently audio streaming
func (a *Audio) IsPlaying() bool {
	return a.isPlaying
}

/////////////////////////////////////////////////
//// Internal stuff
//////////////////////////////////////////////////

func (a *Audio) playback(stream beep.StreamSeekCloser, format beep.Format) {

	a.isPlaying = true
	resampled := beep.Resample(4, format.SampleRate, a.sampleRate, stream)
	cancel := a.cancel.Listen()
	done := make(chan bool)

	speaker.Play(beep.Seq(resampled, beep.Callback(func() {
		done <- true
	})))

	select {
	case <-done:
		a.isPlaying = false
		cancel.Discard()
	case <-cancel.Channel():
		cancel.Discard()
		log.Trace("Kill play")
	}
}

func (a *Audio) ExternalPlayback(stream beep.StreamSeekCloser, format beep.Format) {
	a.playback(stream, format)
	// a.isPlaying = true
	// resampled := beep.Resample(4, format.SampleRate, a.sampleRate, stream)

	// cancel := a.cancel.Listen()
	// done := make(chan bool)
	// speaker.Play(beep.Seq(resampled, beep.Callback(func() {
	// 	done <- true
	// })))

	// select {
	// case <-done:
	// 	a.isPlaying = false
	// 	cancel.Discard()
	// case <-cancel.Channel():
	// 	cancel.Discard()
	// 	log.Trace("Kill play")
	// }
}
