package controller

import (
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/systemet"
)

type SystemetPidMenu struct {
}

func (m *SystemetPidMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	fmt.Println(k)

	k2, err := strconv.Atoi(k)
	if err != nil {
		return MenuReturn{
			NextFunction: menu.Caller,
		}
	}
	s, err := systemet.RequestNewProduct(k2)

	if err != nil {
		return MenuReturn{
			NextFunction: menu.Caller,
		}
	}

	message := ""
	message = message +
		"Artikelnummer: " + s.ProductNumberShort + ", " + s.ProductNameBold + ", " +
		"Kateogri: " + s.Category + ", " +
		"Förpackning: " + s.BottleTextShort + ", " +
		"Volym: " + strconv.FormatFloat(s.Volume, 'f', 2, 64) + " milliliter, " +
		"Alkohol procent: " + strconv.FormatFloat(s.AlcoholPercentage, 'f', 2, 64) + ", " +
		"Pris: " + strconv.FormatFloat(s.Price, 'f', 2, 64) + " kronor, " +
		"Pant: " + strconv.FormatFloat(s.RecycleFee, 'f', 2, 64) + " kronor, " +
		"Typ: " + s.Type + ", " +
		"Stil: " + s.Style + ", " +
		"Användnignsområden: " + s.Usage +
		"Smak: " + s.Taste

	fmt.Println(message)

	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}

	go c.Audio.PlayMP3FromStream(ttsData)

	return MenuReturn{
		NextFunction: menu.Caller,
	}

}

func (m *SystemetPidMenu) InputLength() int {
	return 4
}

func (m *SystemetPidMenu) Name() string {
	return "balance"
}

func (m *SystemetPidMenu) Prefix(c *Controller) {
	message := "Mata in Systembolagets artikelnummer, 5 siffror."
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}

	go c.Audio.PlayMP3FromStream(ttsData)
}
