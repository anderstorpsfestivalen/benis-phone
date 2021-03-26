package controller

import "strconv"

type SystemetArboga struct {
}

func (m *SystemetArboga) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	stock, err := c.SystemetAPI.GetStock("508393", "0611")
	if err != nil {
		return MenuReturn{
			Error:        err,
			NextFunction: "error",
		}
	}

	message := "Antalet Arboga 10.2 i lager på Systembolaget Gislaved är just nu " + strconv.Itoa(stock[0].Stock) + "stycken      ."
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		return MenuReturn{
			Error:        err,
			NextFunction: "error",
		}
	}
	c.Audio.PlayMP3FromStream(ttsData)

	return MenuReturn{
		NextFunction: menu.Caller,
	}

}

func (m *SystemetArboga) InputLength() int {
	return 0
}

func (m *SystemetArboga) Name() string {
	return "systemet"
}

func (m *SystemetArboga) Prefix(c *Controller) {
}
