package controller

import log "github.com/sirupsen/logrus"

type BarIndex struct {
}

func (m *BarIndex) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	switch k {
	case "1":
		//BAR CLOSING
		return MenuReturn{
			NextFunction: "barclosingmenu",
		}
	case "2":
		//CURRENT MENU
		return MenuReturn{
			NextFunction: "currentmenu",
		}
	case "4":
		//BALANCE LOOKUP
		return MenuReturn{
			NextFunction: "balance",
		}
	default:
		return MenuReturn{
			NextFunction: "mainmenu",
		}
	}
}

func (m *BarIndex) InputLength() int {
	return 1
}

func (m *BarIndex) Name() string {
	return "barindex"
}

func (m *BarIndex) Prefix(c *Controller) {
	message := "Tryck 1 för barstängning. 2 för nuvarande meny. 3 för nuvarande saldo. 0 för att gå tillbaka."
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)

}
