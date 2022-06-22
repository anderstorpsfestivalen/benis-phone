package functions

type TTS struct {
	Message  string `toml:"msg"`
	Voice    string
	Language string `toml:"lang"`
	Engine   string
}

func (t *TTS) SetDefault(dv string, dl string, de string) {
	if t.Message != "" {
		if t.Voice == "" {
			t.Voice = dv
		}

		if t.Language == "" {
			t.Language = dl
		}

		if t.Engine == "" {
			t.Engine = de
		}
	}
}

func (t TTS) GetPlayable() (Playable, error) {
	return Playable{
		TTS: t,
	}, nil
}

func (t *Definition) StandardTTS(message string) TTS {
	return TTS{
		Message:  message,
		Voice:    t.General.DefaultTTSVoice,
		Language: t.General.DefaultTTSLanguage,
	}
}

func (t *Definition) EnglishTTS(message string) TTS {
	return TTS{
		Message:  message,
		Voice:    "Kendra",
		Language: "en-US",
	}
}
