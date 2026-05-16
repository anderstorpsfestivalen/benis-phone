package functions

type TTS struct {
	Message  string `toml:"msg"`
	Voice    string
	Language string `toml:"lang"`
	Engine   string

	// Provider selects the TTS backend ("polly", "elevenlabs", ...). Empty
	// falls back to the definition's DefaultTTSProvider, which itself falls
	// back to the registry's default at synthesis time.
	Provider string `toml:"provider"`
}

func (t *TTS) SetDefault(dv, dl, de, dp string) {
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

		if t.Provider == "" {
			t.Provider = dp
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
		Engine:   t.General.DefaultTTSEngine,
		Provider: t.General.DefaultTTSProvider,
	}
}

func (t *Definition) EnglishTTS(message string) TTS {
	return TTS{
		Message:  message,
		Voice:    "Kendra",
		Language: "en-US",
		Engine:   "neural",
		Provider: t.General.DefaultTTSProvider,
	}
}
