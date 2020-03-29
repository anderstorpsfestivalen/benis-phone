package controller

import (
	"fmt"
)

type MainMenu struct {
}

func (m *MainMenu) Run(c *Controller, k string) MenuReturn {

	fmt.Println("RECEIVED: " + k)
	switch k {
	case "1":
		message := "orvars korvar och makaroner"
		//polly.TTS(message, "Astrid")
		fmt.Println(k, message)

		c.Mpd.Add("test.mp3")
		c.Mpd.PlayBlocking()
	case "2":
		message := "penis lasse"
		//polly.TTS(message, "Astrid")
		fmt.Println(k, message)

		c.Mpd.Add("test.mp3")
		c.Mpd.PlayBlocking()
	case "3":
		c.Mpd.Clear()
		return MenuReturn{
			NextAction:   "LUL",
			NextFunction: "announce",
		}
	}

	return MenuReturn{
		NextAction:   "LUL",
		NextFunction: "mainmenu",
	}
}

func (m *MainMenu) InputLength() int {
	return 1
}
