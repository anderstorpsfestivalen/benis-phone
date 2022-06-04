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
	Service    Service    `toml:"srv"`
	Gate       Gate       `toml:"gate"`
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

	if a.Service.Destination != "" {
		return "srv", nil
	}

	if a.Gate != (Gate{}) {
		return "gate", nil
	}

	if a.Clear {
		return "clear", nil
	}

	return "", fmt.Errorf("cannot determine action type")
}
