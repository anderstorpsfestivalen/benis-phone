package functions

import "fmt"

type Action struct {
	Num   int
	Wait  bool
	Clear bool

	//////////////
	// actionables
	//////////////

	// Play something before triggering the action
	Prefix Prefix `toml:"prefix"`
	// Links to another menu
	Dst string
	// Plays a file (mp3, ogg, etc)
	File File `toml:"file"`
	// Plays a random file from a folder
	RandomFile RandomFile `toml:"randomfile"`
	// Reads a TTS
	TTS TTS `toml:"tts"`
	// Calls a service
	Service Service `toml:"srv"`
	// Redirects to a custom dispatcher
	CustomDispatcher string `toml:"dispatcher"`
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

	if a.CustomDispatcher != "" {
		return "dispatcher", nil
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
