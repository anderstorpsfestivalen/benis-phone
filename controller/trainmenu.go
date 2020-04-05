package controller

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/polly"
	"gitlab.com/anderstorpsfestivalen/benis-phone/train"
)

type TrainMenu struct {
}

func (m *TrainMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	c.Mpd.Clear()
	message := train.Get()
	fmt.Println(message)
	polly.TTS(message, "Astrid")
	c.Mpd.Add("test.mp3")
	c.Mpd.PlayBlocking()

	return MenuReturn{
		NextFunction: menu.Caller,
	}

}

func (m *TrainMenu) InputLength() int {
	return 0
}

func (m *TrainMenu) Name() string {
	return "trainmenu"
}
