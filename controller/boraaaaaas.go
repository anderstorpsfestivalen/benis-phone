package controller

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

type Boraaaaaas struct{}

func (m *Boraaaaaas) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case key := <-keychan:
			if key == "0" {
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			} else {
				fmt.Println("pressed " + key)
				go c.Audio.PlayFromFile("files/boraaaaaas.ogg")
			}
		}
	}
}
func (m *Boraaaaaas) InputLength() int {
	return 0
}

func (m *Boraaaaaas) Name() string {
	return "boraaaaaas"
}

func (m *Boraaaaaas) Prefix(c *Controller) {
	message := "TRYCK, ETT, till FYRKANT, NOLL FÖR ATT GÅ TILLBAKA"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
