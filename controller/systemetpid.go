package controller

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

type SystemetPid struct {
}

func (m *SystemetPid) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	res, err := c.SystemetAPI.SearchForItem(k)
	if err != nil {
		return MenuReturn{
			Error:        err,
			NextFunction: "error",
		}
	}

	message := ""
	message = message + "Artikelnummer: " + res.Products[0].ProducerName
	//ProductNumberShort + ", " + s.ProductNameBold + ", " +
	// 	"Kateogri: " + s.Category + ", " +
	// 	"Förpackning: " + s.BottleTextShort + ", " +
	// 	"Volym: " + strconv.FormatFloat(s.Volume, 'f', 0, 64) + " milliliter, " +
	// 	"Alkohol procent: " + strconv.FormatFloat(s.AlcoholPercentage, 'f', 1, 64) + ", " +
	// 	"Pris: " + strconv.FormatFloat(s.Price, 'f', 0, 64) + " kronor, " +
	// 	"Pant: " + strconv.FormatFloat(s.RecycleFee, 'f', 0, 64) + " krona, " +
	// 	"Typ: " + s.Type + ", " +
	// 	"Stil: " + s.Style + ", " +
	// 	"Användnignsområden: " + s.Usage +
	// 	"Smak: " + s.Taste

	fmt.Println(message)

	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}

	c.Audio.PlayMP3FromStream(ttsData)

	return MenuReturn{
		NextFunction: menu.Caller,
	}

}

func (m *SystemetPid) InputLength() int {
	return 5
}

func (m *SystemetPid) Name() string {
	return "balance"
}

func (m *SystemetPid) Prefix(c *Controller) {
	message := "Mata in Systembolagets artikelnummer, 5 siffror."
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}

	go c.Audio.PlayMP3FromStream(ttsData)
}
