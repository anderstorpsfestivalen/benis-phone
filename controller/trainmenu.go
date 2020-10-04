package controller

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/services/train"
)

type TrainMenu struct {
}

func (m *TrainMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	message := train.Get()
	fmt.Println(message)
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		return MenuReturn{
			Error:        err,
			NextFunction: "error",
		}
	}
	c.Audio.PlayMP3FromStream(ttsData)

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

func (m *TrainMenu) Prefix(c *Controller) {
}
