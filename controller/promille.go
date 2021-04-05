package controller

import (
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/backend"
)

type Promille struct {
}

func (m *Promille) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	promille, err := backend.GetPromilleForPhoneNumber(k)
	_ = promille
	if err != nil {
		ttsData, err := c.Polly.TTS("Telefonnummret kan inte hittas, var god försök igen.", "Astrid")
		if err != nil {
			log.Error(err)
		}
		c.Audio.PlayMP3FromStream(ttsData)

		return MenuReturn{
			NextFunction: menu.Caller,
		}
	}

	message := promille.Name + ". Din uppskattade promille är: " +
		strconv.FormatFloat(promille.Promille, 'f', 1, 64)

	fmt.Println(message)

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

func (m *Promille) InputLength() int {
	return 10
}

func (m *Promille) Name() string {
	return "promille"
}

func (m *Promille) Prefix(c *Controller) {
	message := "Fyll i ditt telefonnummer, tio siffror."
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}

	go c.Audio.PlayMP3FromStream(ttsData)
}
