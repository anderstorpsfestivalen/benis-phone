package controller

import (
	log "github.com/sirupsen/logrus"
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/backend"
)

type Balance struct {
}

func (m *Balance) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	balance, err := backend.GetBalanceForPhoneNumber(k)
	_ = balance
	if err != nil {
		return MenuReturn{
			Error:        err,
			NextFunction: "error",
		}
	}

	return MenuReturn{
		NextFunction: menu.Caller,
	}

}

func (m *Balance) InputLength() int {
	return 10
}

func (m *Balance) Name() string {
	return "balance"
}

func (m *Balance) Prefix(c *Controller) {
	message := "Fyll i ditt telefonnummer, avsluta med fyrkant."
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}

	go c.Audio.PlayMP3FromStream(ttsData)
}
