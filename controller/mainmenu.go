package controller

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/polly"
)

type MainMenu struct {
}

func (m *MainMenu) Run(c *Controller, k string) MenuReturn {

	fmt.Println("RECEIVED: " + k)
	switch k {
	case "1":
		message := "orvars korvar och makaroner"
		polly.TTS(message, "Astrid")
		fmt.Println(k, message)

		c.Mpd.Add("test.mp3")
		c.Mpd.Play()
	case "2":
		message := "penis lasse"
		polly.TTS(message, "Astrid")
		fmt.Println(k, message)

		c.Mpd.Add("test.mp3")
		c.Mpd.Play()
	}

	return MenuReturn{
		NextAction:   "LUL",
		NextFunction: "mainmenu",
	}
}

func (m *MainMenu) InputLength() int {
	return 1
}

func (m *MainMenu) Name() string {
	return "mainmenu"
}
