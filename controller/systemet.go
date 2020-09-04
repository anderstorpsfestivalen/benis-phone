package controller

import (
	"gitlab.com/anderstorpsfestivalen/benis-phone/systemet"
)

type Systemet struct {
}

func (m *Systemet) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	c.Mpd.Clear()
	stock, err := systemet.RequestStockData("508393")
	if err != nil {
		return MenuReturn{
			NextFunction: "mainmenu",
		}
	}
	message := "Antalet Arboga 10.2 i lager på Systembolaget Gislaved är just nu " + stock.StockTextShort
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

func (m *Systemet) InputLength() int {
	return 0
}

func (m *Systemet) Name() string {
	return "systemet"
}
