package controller

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/backend"
)

type Balance struct {
}

func (m *Balance) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	fmt.Println(k)
	balance, err := backend.GetBalanceForPhoneNumber(k)
	if err != nil {
		return MenuReturn{
			NextFunction: menu.Caller,
		}
	}

	fmt.Println(balance, err)
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
