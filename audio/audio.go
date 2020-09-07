package audio

import (
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
)

type Audio struct {
	sampleRate beep.SampleRate
}

func New(samplerate int) *Audio {
	return &Audio{
		sampleRate: beep.SampleRate(samplerate),
	}
}

func (a *Audio) Init() error {
	err := speaker.Init(a.sampleRate, a.sampleRate.N(time.Second/30))
	if err != nil {
		return err
	}
	return nil
}
