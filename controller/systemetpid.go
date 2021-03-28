package controller

import (
	"fmt"
	"strconv"

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
	message = message +
		"Artikelnummer: " + res.Products[0].ProductNumberShort +
		", " + res.Products[0].ProductNameBold + ", " +
		"Producent: " + res.Products[0].ProducerName + ", " +
		"Kategori: " + res.Products[0].CategoryLevel1 + ", " +
		"Förpackning: " + res.Products[0].BottleTextShort + ", " +
		"Volym: " + strconv.FormatFloat(res.Products[0].Volume, 'f', 0, 64) + " milliliter, " +
		"Alkohol procent: " + strconv.FormatFloat(res.Products[0].AlcoholPercentage, 'f', 1, 64) + ", " +
		"Pris: " + strconv.FormatFloat(res.Products[0].Price, 'f', 0, 64) + " kronor, " +
		"Pant: " + strconv.FormatFloat(res.Products[0].RecycleFee, 'f', 0, 64) + " krona, " +
		"Användnignsområden: " + res.Products[0].Usage +
		"Smak: " + res.Products[0].Taste + ", " +
		"Färg: " + res.Products[0].Color + ", " +
		// test of taste clocks
		"Passar bra till: "
	for i := range res.Products[0].TasteSymbols {
		message = message + res.Products[0].TasteSymbols[i] + ", "
	}

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
