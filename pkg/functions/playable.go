package functions

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/audio"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/polly"
)

type Playable struct {
	T    string
	File string

	TTSVoice   string
	TTSMessage string
	TTSLang    string

	Wait  bool
	Clear bool
}

func CreatePlayableTTS(message string, voice string, lang string) Playable {
	return Playable{
		T:          "msg",
		TTSVoice:   voice,
		TTSMessage: message,
		TTSLang:    lang,
	}
}

func (p *Playable) Play(a *audio.Audio, polly polly.Polly, defaultTTSVoice string, defaultTTSLang string) error {
	if p.Clear {
		a.Clear()
	}

	voice := defaultTTSVoice

	if p.TTSVoice != "" {
		voice = p.TTSVoice
	}

	if p.T == "file" {
		if p.Wait {
			return a.PlayFromFile(p.File)
		} else {
			go a.PlayFromFile(p.File)
		}

	} else if p.T == "msg" {
		ttsData, err := polly.TTSLang(p.TTSMessage, voice)
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
