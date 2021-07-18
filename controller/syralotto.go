package controller

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

type SyraLotto struct{}

func (m *SyraLotto) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	sub := c.Subscribe(m.Name())
	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case <-sub.Cancel:
			c.Unsubscribe(m.Name())
			return MenuReturn{
				NextFunction: menu.Caller,
			}
		case key := <-keychan:
			switch key {
			case "1":
				fmt.Println("pressed 1")
				go c.Audio.PlayFromFile("files/lasse-liten/e.ogg")
			case "2":
				fmt.Println("pressed 2")
				go c.Audio.PlayFromFile("files/lasse-liten/lsd.ogg")
			case "3":
				fmt.Println("pressed 3")
				go c.Audio.PlayFromFile("files/lasse-liten/acid-house.ogg")
			case "4":
				fmt.Println("pressed 4")
				go c.Audio.PlayFromFile("files/lasse-liten/goa-trance.ogg")
			case "5":
				fmt.Println("pressed 5")
				go c.Audio.PlayFromFile("files/lasse-liten/electro.ogg")
			case "6":
				fmt.Println("pressed 6")
				go c.Audio.PlayFromFile("files/lasse-liten/garage.ogg")
			case "7":
				fmt.Println("pressed 7")
				go c.Audio.PlayFromFile("files/lasse-liten/deep-house.ogg")
			case "8":
				fmt.Println("pressed 8")
				go c.Audio.PlayFromFile("files/lasse-liten/e-type.ogg")
			case "9":
				fmt.Println("pressed 9")
				go c.Audio.PlayFromFile("files/lasse-liten/allt-snurrar.ogg")
			case "*":
				fmt.Println("pressed *")
				go c.Audio.PlayFromFile("files/lasse-liten/torr-i-munnen.ogg")
			case "#":
				fmt.Println("pressed #")
				go c.Audio.PlayFromFile("files/lasse-liten/josses-vad-det-gar-igang.ogg")
			default:
				return MenuReturn{
					NextFunction: menu.Caller,
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
	message := "TRYCK ETT till FYRKANT, NOLL FÖR ATT GÅ TILLBAKA"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
