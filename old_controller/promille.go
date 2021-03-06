package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/backend"
	log "github.com/sirupsen/logrus"
)

type Promille struct {
}

func (m *Promille) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	promille, err := backend.GetPromilleForPhoneNumber(k)
	_ = promille
	if err != nil {
		if err.Error() == "no transactions" {

			ttsData, err := c.Polly.TTS("Inga transaktioner hittade, gå och köp något i baren.", "Astrid")
			if err != nil {
				log.Error(err)
			}
			c.Audio.PlayMP3FromStream(ttsData)

			return MenuReturn{
				NextFunction: menu.Caller,
			}
		}
		ttsData, err := c.Polly.TTS("Telefonnummret kan inte hittas, var god försök igen.", "Astrid")
		if err != nil {
			log.Error(err)
		}
		c.Audio.PlayMP3FromStream(ttsData)

		return MenuReturn{
			NextFunction: "mainmenu",
		}
	}

	p := strconv.FormatFloat(promille.Promille, 'f', 2, 64)
	message := promille.Name + ". Din uppskattade promille är: ." + strings.ReplaceAll(p, ".", ",")

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
		NextFunction: "mainmenu",
	}

}

func (m *Promille) InputLength() int {
	return 10
}

func (m *Promille) Name() string {
	return "promille"
}

func (m *Promille) Prefix(c *Controller) {
	message := "Fyll i ditt telefonnummer, tio siffror. Avsluta med fyrkant."
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}

	go c.Audio.PlayMP3FromStream(ttsData)
}
