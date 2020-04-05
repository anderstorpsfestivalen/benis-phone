package controller

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/polly"
	"gitlab.com/anderstorpsfestivalen/benis-phone/train"
)

type MainMenu struct {
}

func (m *MainMenu) Run(c *Controller, k string) MenuReturn {

	fmt.Println("RECEIVED: " + k)
	switch k {
	case "1":
		c.Mpd.Clear()
		message := "orvars korvar och makaroner"
		polly.TTS(message, "Astrid")
		fmt.Println(k, message)

		c.Mpd.Add("test.mp3")
		c.Mpd.PlayBlocking()
	case "2":
		c.Mpd.Clear()
		message := "penis lasse"
		polly.TTS(message, "Astrid")
		fmt.Println(k, message)

		c.Mpd.Add("test.mp3")
		c.Mpd.PlayBlocking()
	case "3":
		c.Mpd.Clear()
		return MenuReturn{
			NextAction:   "LUL",
			NextFunction: "announce",
		}
	case "4":
		c.Mpd.Clear()
		message := train.Get()
		fmt.Println(message)
		polly.TTS(message, "Astrid")
		c.Mpd.Add("test.mp3")
		c.Mpd.PlayBlocking()
	}
	return MenuReturn{
		NextAction:   "LUL",
		NextFunction: "mainmenu",
	}

}

func (m *MainMenu) InputLength() int {
	return 1
}
