package controller

import (
	"fmt"
	"sync"

	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/audio"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/definition"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/phone"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/polly"
)

type Controller struct {
	Phone      phone.FlowPhone
	Audio      *audio.Audio
	Recorder   audio.Recorder
	Polly      polly.Polly
	Definition definition.Definition
}

func New(ph phone.FlowPhone, audio *audio.Audio, rec audio.Recorder, polly polly.Polly, def definition.Definition) Controller {
	return Controller{
		Phone:      ph,
		Audio:      audio,
		Recorder:   rec,
		Polly:      polly,
		Definition: def,
	}
}

func (c *Controller) Start(wg *sync.WaitGroup) {

	fmt.Println(c.Definition)
}
