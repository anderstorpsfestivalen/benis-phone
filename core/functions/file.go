package functions

import (
	"errors"
	"os"
)

type File struct {
	Source string `toml:"src"`
	Block  bool   `toml:"block"`
	Clear  bool   `toml:"clear"`
}

func (f File) GetPlayable() (Playable, error) {
	if _, err := os.Stat(f.Source); errors.Is(err, os.ErrNotExist) {
		return Playable{}, err
	}

	return Playable{
		File: f,
	}, nil
}
