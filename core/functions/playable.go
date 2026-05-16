package functions

import (
	"fmt"

	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/tts"
	log "github.com/sirupsen/logrus"
)

type Playable struct {
	File File
	TTS  TTS

	Wait  bool
	Clear bool
}

func CreatePlayable(p PlayGenerator) Playable {

	pl, err := p.GetPlayable()
	if err != nil {
		log.Error(err)
	}

	return pl
}

func (p *Playable) Play(a audio.AudioSink, ttsReg *tts.Registry) error {
	if p.Clear {
		a.Clear()
	}

	if p.File != (File{}) {
		if p.Wait {
			return a.PlayFromFile(p.File.Source)
		} else {
			go a.PlayFromFile(p.File.Source)
			return nil
		}

	} else if p.TTS != (TTS{}) {
		req := tts.Request{
			Message:  p.TTS.Message,
			Voice:    p.TTS.Voice,
			Language: p.TTS.Language,
			Engine:   p.TTS.Engine,
		}
		ttsData, err := ttsReg.Synthesize(p.TTS.Provider, req)
		if err != nil {
			return err
		}

		log.WithFields(log.Fields{"message": p.TTS.Message,
			"lang":     p.TTS.Language,
			"voice":    p.TTS.Voice,
			"engine":   p.TTS.Engine,
			"provider": p.TTS.Provider}).Info("Playing TTS")

		if p.Wait {
			return a.PlayFromStream(ttsData)
		} else {
			go a.PlayFromStream(ttsData)
		}
		return nil
	}

	if p.Clear {
		return nil
	}

	return fmt.Errorf("Playable type not defined")
}

func (p *Playable) Type() (string, error) {
	if p.File != (File{}) {
		return "file", nil
	}

	if p.TTS != (TTS{}) {
		return "tts", nil
	}

	return "", fmt.Errorf("could not determine type of playable")
}

type PlayGenerator interface {
	GetPlayable() (Playable, error)
}
