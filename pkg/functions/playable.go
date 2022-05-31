package functions

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/audio"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/polly"
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

func (p *Playable) Play(a *audio.Audio, polly polly.Polly) error {
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
		ttsData, err := polly.TTSLang(p.TTS.Message, p.TTS.Language, p.TTS.Voice)
		if err != nil {
			return err
		}

		if p.Wait {
			return a.PlayMP3FromStream(ttsData)
		} else {
			go a.PlayMP3FromStream(ttsData)
		}
		return nil
	}

	if p.Clear {
		return nil
	}

	return fmt.Errorf("Playable type not defined")
}

type PlayGenerator interface {
	GetPlayable() (Playable, error)
}
