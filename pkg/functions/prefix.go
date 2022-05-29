package functions

import (
	"fmt"
)

type Prefix struct {
	File        File `toml:"file"`
	TTS         TTS  `toml:"tts"`
	Wait        bool
	IgnoreClear bool
}

func (p *Prefix) GetPlayable() (Playable, error) {

	// Check if prefix is empty
	if p.File == (File{}) && p.TTS == (TTS{}) {
		return Playable{}, nil
	}
	// Check if prefix is double defined
	if p.File != (File{}) && p.TTS != (TTS{}) {
		return Playable{}, fmt.Errorf("Prefix cannot have both a file and a message. ", p.File, p.TTS.Message)
	}

	return Playable{
		File:  p.File,
		TTS:   p.TTS,
		Wait:  p.Wait,
		Clear: !p.IgnoreClear,
	}, nil

}
