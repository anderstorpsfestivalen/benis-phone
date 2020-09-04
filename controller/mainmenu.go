package controller

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

type MainMenu struct {
}

func (m *MainMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	fmt.Println("RECEIVED: " + k)
	switch k {
	case "1":
		c.Mpd.Clear()
		message := "orvars korvar och makaroner"
		filename, err := c.Polly.TTS(message, "Astrid")
		if err != nil {
			log.Error(err)
			return MenuReturn{
				NextFunction: "mainmenu",
			}
		}

		c.Mpd.Add(filename)
		c.Mpd.PlayBlocking()
	case "2":
		c.Mpd.Clear()
		message := "penis lasse"
		filename, err := c.Polly.TTS(message, "Astrid")
		if err != nil {
			log.Error(err)
			return MenuReturn{
				NextFunction: "mainmenu",
			}
		}

		c.Mpd.Add(filename)
		c.Mpd.PlayBlocking()
	case "3":
		c.Mpd.Clear()
		return MenuReturn{
			NextFunction: "announce",
		}
	case "4":
		c.Mpd.Clear()
		return MenuReturn{
			NextFunction: "trainmenu",
		}
	}
	return MenuReturn{
		NextFunction: "mainmenu",
	}

}

func (m *MainMenu) InputLength() int {
	return 1
}

func (m *MainMenu) Name() string {
	return "mainmenu"
}
