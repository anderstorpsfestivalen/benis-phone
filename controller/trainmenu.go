package controller

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/train"
)

type TrainMenu struct {
}

func (m *TrainMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	c.Mpd.Clear()
	message := train.Get()
	fmt.Println(message)
	filename, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		return MenuReturn{
			NextFunction: "mainmenu",
		}
	}
	c.Mpd.Add(filename)
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
