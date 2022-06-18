package controller

import (
	"fmt"

	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/train"
)

type TrainMenu struct {
}

func (m *TrainMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	tr := train.Train{}
	message, err := tr.Get("")
	if err != nil {
		panic(err)
	}
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
		NextFunction: "mainmenu",
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
