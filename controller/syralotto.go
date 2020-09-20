package controller

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

type SyraLotto struct{}

func (m *SyraLotto) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case key := <-keychan:
			if key == "1" {
				fmt.Println("pressed 1")
				go c.Audio.PlayFromFile("files/lasse-liten/e.ogg")
			} else if key == "2" {
				fmt.Println("pressed 2")
				go c.Audio.PlayFromFile("files/lasse-liten/lsd.ogg")
			} else if key == "3" {
				fmt.Println("pressed 3")
				go c.Audio.PlayFromFile("files/lasse-liten/acid-house.ogg")
			} else if key == "4" {
				fmt.Println("pressed 4")
				go c.Audio.PlayFromFile("files/lasse-liten/goa-trance.ogg")
			} else if key == "5" {
				fmt.Println("pressed 5")
				go c.Audio.PlayFromFile("files/lasse-liten/electro.ogg")
			} else if key == "6" {
				fmt.Println("pressed 6")
				go c.Audio.PlayFromFile("files/lasse-liten/garage.ogg")
			} else if key == "7" {
				fmt.Println("pressed 7")
				go c.Audio.PlayFromFile("files/lasse-liten/deep-house.ogg")
			} else if key == "8" {
				fmt.Println("pressed 8")
				go c.Audio.PlayFromFile("files/lasse-liten/e-type.ogg")
			} else if key == "9" {
				fmt.Println("pressed 9")
				go c.Audio.PlayFromFile("files/lasse-liten/allt-snurrar.ogg")
			} else if key == "*" {
				fmt.Println("pressed *")
				go c.Audio.PlayFromFile("files/lasse-liten/torr-i-munnen.ogg")
			} else if key == "#" {
				fmt.Println("pressed #")
				go c.Audio.PlayFromFile("files/lasse-liten/josses-vad-det-gar-igang.ogg")
			} else {
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			}
		}
	}
}
func (m *SyraLotto) InputLength() int {
	return 0
}

func (m *SyraLotto) Name() string {
	return "syralotto"
}

func (m *SyraLotto) Prefix(c *Controller) {
	message := "TRYCK 1 till FYRKANT"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
