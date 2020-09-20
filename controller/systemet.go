package controller

import (
	"strings"

	"gitlab.com/anderstorpsfestivalen/benis-phone/services/systemet"
)

type Systemet struct {
}

func (m *Systemet) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	stock, err := systemet.RequestStockData("508393")
	if err != nil {
		return MenuReturn{
			NextFunction: "mainmenu",
		}
	}
	message := "Antalet Arboga 10.2 i lager på Systembolaget Gislaved är just nu " + strings.Replace(stock.StockTextShort, "st", "stycken     .", -1)
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

func (m *Systemet) InputLength() int {
	return 0
}

func (m *Systemet) Name() string {
	return "systemet"
}
