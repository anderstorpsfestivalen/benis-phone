package controller

import (
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/backend"
)

type FulolPoints struct {
}

func (m *FulolPoints) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	fulolpoints, err := backend.GetFulolPointsForPhoneNumber(k)
	_ = fulolpoints
	if err != nil {
		ttsData, err := c.Polly.TTS("Telefonnummret kan inte hittas, var god försök igen.", "Astrid")
		if err != nil {
			log.Error(err)
		}
		c.Audio.PlayMP3FromStream(ttsData)

		return MenuReturn{
			NextFunction: "mainmenu",
		}
	}

	message := fulolpoints.Name + ". Din fulöls poäng är: " +
		strconv.FormatFloat(fulolpoints.Points, 'f', 0, 64) +
		". Poäng."

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

func (m *FulolPoints) InputLength() int {
	return 10
}

func (m *FulolPoints) Name() string {
	return "fulolpoints"
}

func (m *FulolPoints) Prefix(c *Controller) {
	message := "Fyll i ditt telefonnummer, tio siffror. Avsluta med fyrkant."
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}

	go c.Audio.PlayMP3FromStream(ttsData)
}
