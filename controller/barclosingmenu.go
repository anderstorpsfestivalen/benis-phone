package controller

import (
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/barclosing"
)

type BarClosingMenu struct {
}

func (m *BarClosingMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	message := barclosing.ClosingTime()

	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		return MenuReturn{
			NextFunction: "mainmenu",
		}
	}
	c.Audio.PlayMP3FromStream(ttsData)

	return MenuReturn{
		NextFunction: menu.Caller,
	}

}

func (m *BarClosingMenu) InputLength() int {
	return 0
}

func (m *BarClosingMenu) Name() string {
	return "barclosingmenu"
}
