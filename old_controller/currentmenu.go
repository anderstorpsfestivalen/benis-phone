package controller

import (
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/currentmenu"
)

type CurrentMenu struct {
}

func (m *CurrentMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	message, err := currentmenu.ListItems()
	if err != nil {
		return MenuReturn{
			Error:        err,
			NextFunction: "error",
		}
	}

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

func (m *CurrentMenu) InputLength() int {
	return 0
}

func (m *CurrentMenu) Name() string {
	return "currentmenu"
}

func (m *CurrentMenu) Prefix(c *Controller) {
}
