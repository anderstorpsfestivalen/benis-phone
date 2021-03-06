package functions

import (
	"fmt"

	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/polly"
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
		ttsData, err := polly.TTSLang(p.TTS.Message, p.TTS.Language, p.TTS.Voice, p.TTS.Engine)
		if err != nil {
			return err
		}

		log.WithFields(log.Fields{"message": p.TTS.Message,
			"lang":   p.TTS.Language,
			"voice":  p.TTS.Voice,
			"engine": p.TTS.Engine}).Info("Playing TTS")

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
