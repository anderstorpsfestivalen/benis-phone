package controller

import (
	log "github.com/sirupsen/logrus"
)

type BarMenu struct{}

func (m *BarMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case key := <-keychan:
			if key == "1" {
				return MenuReturn{
					NextFunction: "currentmenu",
				}
			} else if key == "2" {
				return MenuReturn{
					NextFunction: "barclosingmenu",
				}
			} else {
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			}
		}
	}
}
func (m *BarMenu) InputLength() int {
	return 0
}

func (m *BarMenu) Name() string {
	return "barmenu"
}

func (m *BarMenu) Prefix(c *Controller) {
	message := "Tryck ett, för nuvarande meny, tryck två, för att få veta när baren stänger, för att återgå, tryck 0"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
