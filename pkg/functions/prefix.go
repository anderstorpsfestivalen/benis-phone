package functions

import (
	"fmt"
)

type Prefix struct {
	File        string
	Message     string `toml:"msg"`
	TTSVoice    string `toml:"voice"`
	Wait        bool
	IgnoreClear bool
}

func (p *Prefix) GetPlayable() (Playable, error) {
	// Check if prefix is empty
	if p.File == "" && p.Message == "" {
		return Playable{}, nil
	}
	// Check if prefix is double defined
	if p.File != "" && p.Message != "" {
		return Playable{}, fmt.Errorf("Prefix cannot have both a file and a message. ", p.File, p.Message)
	}

	t := "file"
	if p.Message != "" {
		t = "msg"
	}

	return Playable{
		T:          t,
		File:       p.File,
		TTSMessage: p.Message,
		TTSVoice:   p.TTSVoice,
		Wait:       p.Wait,
		Clear:      !p.IgnoreClear,
	}, nil

}
