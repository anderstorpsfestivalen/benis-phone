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
		message := "orvars korvar och makaroner"
		ttsData, err := c.Polly.TTS(message, "Astrid")
		if err != nil {
			log.Error(err)
			return MenuReturn{
				NextFunction: "mainmenu",
			}
		}
		c.Audio.PlayMP3FromStream(ttsData)

	case "2":
		message := "penis lasse"
		ttsData, err := c.Polly.TTS(message, "Astrid")
		if err != nil {
			log.Error(err)
			return MenuReturn{
				NextFunction: "mainmenu",
			}
		}

		c.Audio.PlayMP3FromStream(ttsData)

	case "3":
		return MenuReturn{
			NextFunction: "barclosingmenu",
		}
	case "4":
		return MenuReturn{
			NextFunction: "trainmenu",
		}
	case "5":
		return MenuReturn{
			NextFunction: "systemet",
		}
	case "6":
		c.Audio.PlayFromFile("files/alex tuuta.mp3")

	case "7":
		return MenuReturn{
			NextFunction: "syralotto",
		}
	case "R":
		return MenuReturn{
			NextFunction: "announce",
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
