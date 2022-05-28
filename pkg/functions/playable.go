package functions

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/audio"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/polly"
)

type Playable struct {
	File string
	TTS  TTS

	Wait  bool
	Clear bool
}

func CreatePlayableFromTTS(t TTS) Playable {
	return Playable{
		TTS: t,
	}
}

func (p *Playable) Play(a *audio.Audio, polly polly.Polly) error {
	if p.Clear {
		a.Clear()
	}

	if p.File != "" {
		if p.Wait {
			return a.PlayFromFile(p.File)
		} else {
			go a.PlayFromFile(p.File)
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
	} else {
		return fmt.Errorf("Playable type not defined")
	}

	return nil
}
