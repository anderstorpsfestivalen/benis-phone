package controller

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/barclosing"
)

type BarClosingMenu struct {
}

func (m *BarClosingMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	fmt.Println("testing1")
	c.Mpd.Clear()
	fmt.Println("testing2")
	message := barclosing.Closing()
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

func (m *BarClosingMenu) InputLength() int {
	return 0
}

func (m *BarClosingMenu) Name() string {
	return "barclosingmenu"
}
