package controller

import (
	log "github.com/sirupsen/logrus"
)

type ExtraMenu struct{}

func (m *ExtraMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case key := <-keychan:
			if key == "1" {
				return MenuReturn{
					NextFunction: "perrralotto",
				}
			} else if key == "2" {
				return MenuReturn{
					NextFunction: "drogslanglotto",
				}
			} else {
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			}
		}
	}
}
func (m *ExtraMenu) InputLength() int {
	return 0
}

func (m *ExtraMenu) Name() string {
	return "extramenu"
}

func (m *ExtraMenu) Prefix(c *Controller) {
	message := "Välkommen till extra menyn, för perrra, tryck ett, för drog slang, tryck två, för att gå tillbaka, tryck 0"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
