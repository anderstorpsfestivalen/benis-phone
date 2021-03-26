package controller

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

type PersonalMenu struct{}

func (m *PersonalMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case key := <-keychan:
			if key == "1" {
				fmt.Println("DEBUG: personal menu, 1 pressed")
				message := "Mata in ditt telefonnummer, avsluta med #"
				ttsData, err := c.Polly.TTS(message, "Astrid")
				if err != nil {
					log.Error(err)
				}
				c.Audio.PlayMP3FromStream(ttsData)

				// röv

				return MenuReturn{
					NextFunction: "balance",
				}
			} else if key == "2" {
				return MenuReturn{
					NextFunction: "", //to be promille koll
				}
			} else {
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			}
		}
	}
}
func (m *PersonalMenu) InputLength() int {
	return 0
}

func (m *PersonalMenu) Name() string {
	return "personalmenu"
}

func (m *PersonalMenu) Prefix(c *Controller) {
	message := "Välkommen till din personliga service meny, för saldo, tryck ett, för promille, tryck två, för att gå tillbaka, tryck #"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
