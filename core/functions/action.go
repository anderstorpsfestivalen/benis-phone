package functions

import "fmt"

type Action struct {
	Num   int
	Dst   string
	Wait  bool
	Clear bool

	// actionables

	File       File       `toml:"file"`
	RandomFile RandomFile `toml:"randomfile"`
	TTS        TTS        `toml:"tts"`
	Service    Service    `toml:"srv"`
	Prefix     Prefix     `toml:"prefix"`
}

func (a *Action) Type() (string, error) {
	if a.Dst != "" {
		return "fn", nil
	}

	if a.File != (File{}) {
		return "file", nil
	}

	if a.RandomFile != (RandomFile{}) {
		return "randomfile", nil
	}

	if a.TTS != (TTS{}) {
		return "tts", nil
	}

	if a.Service.Destination != "" {
		return "srv", nil
	}

	if a.Clear {
		return "clear", nil
	}

	return "", fmt.Errorf("cannot determine action type")
}

func (a *Action) GetPrefix() (Prefix, error) {
	if a.Prefix != (Prefix{}) {
		return a.Prefix, nil
	}

	return Prefix{}, fmt.Errorf("no prefix")
}
