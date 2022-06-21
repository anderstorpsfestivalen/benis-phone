package functions

import (
	"fmt"
	"os"

	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	wr "github.com/mroth/weightedrand"
)

type Queue struct {
	Name string

	EntryMessage    Playable `toml:"entrymsg"`
	Min             int
	Max             int
	Prompts         []QueuePrompt `toml:"prompt"`
	BackgroundMusic File          `toml:"bgmusic"`
	End             Action

	rm       *wr.Chooser
	lastPos  int
	streamer beep.StreamSeekCloser
}

type QueuePrompt struct {
	Prompt Playable `toml:"prompt"`
	Weight int
}

func (q *Queue) Load() error {

	var ch []wr.Choice

	for _, c := range q.Prompts {
		ch = append(ch, wr.NewChoice(c.Prompt, uint(c.Weight)))
	}

	chooser, err := wr.NewChooser(ch...)

	if err != nil {
		return err
	}

	q.rm = chooser

	return nil
}

func (q *Queue) Start() {
	// fmt.Println(rand.Intn(max - min) + min)

}

func (q *Queue) Stop() {

}

func (q *Queue) StartBackground(a *audio.Audio) {
	f, _ := os.Open(q.BackgroundMusic.Source)

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		fmt.Println("penis")
	}
	q.streamer = streamer
	q.streamer.Seek(q.lastPos)

	go a.ExternalPlayback(q.streamer, format)

}
