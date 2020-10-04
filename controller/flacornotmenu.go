package controller

import (
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/flacornot"
)

type FlacOrNotMenu struct {
}

func (m *FlacOrNotMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	message, err := flacornot.FlacOrNot()
	if err != nil {
		return MenuReturn{
			Error:        err,
			NextFunction: "error",
		}
	}

	ttsData, err := c.Polly.TTSLang(message, "en-US", "Joanna")
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

func (m *FlacOrNotMenu) InputLength() int {
	return 0
}

func (m *FlacOrNotMenu) Name() string {
	return "flacornotmenu"
}

func (m *FlacOrNotMenu) Prefix(c *Controller) {
}
