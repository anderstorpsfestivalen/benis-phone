package controller

import (
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/barclosing"
)

type BarClosingMenu struct {
}

func (m *BarClosingMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	message := barclosing.ClosingTime()

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

func (m *BarClosingMenu) InputLength() int {
	return 0
}

func (m *BarClosingMenu) Name() string {
	return "barclosingmenu"
}

func (m *BarClosingMenu) Prefix(c *Controller) {
}
