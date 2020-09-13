package controller

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/currentmenu"
)

type CurrentMenu struct {
}

func (m *CurrentMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	message, err := currentmenu.ListItems()
	if err != nil {
		fmt.Println(err)
	}

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

func (m *CurrentMenu) InputLength() int {
	return 0
}

func (m *CurrentMenu) Name() string {
	return "currentmenu"
}
