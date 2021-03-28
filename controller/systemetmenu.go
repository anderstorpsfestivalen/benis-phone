package controller

import (
	log "github.com/sirupsen/logrus"
)

type SystemetMenu struct{}

func (m *SystemetMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case key := <-keychan:
			if key == "1" {
				return MenuReturn{
					NextFunction: "systemetarboga",
				}
			} else if key == "2" {
				return MenuReturn{
					NextFunction: "systemetpid",
				}
			} else {
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			}
		}
	}
}
func (m *SystemetMenu) InputLength() int {
	return 0
}

func (m *SystemetMenu) Name() string {
	return "systemetmenu"
}

func (m *SystemetMenu) Prefix(c *Controller) {
	message := "Tryck ett, för antalet arboga 10 komma 2 i lager på systembolaget gislaved, tryck två, systembolaget produkt sök, för att återgå, tryck #"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}