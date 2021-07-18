package controller

import (
	log "github.com/sirupsen/logrus"
)

type ExtraMenu struct{}

func (m *ExtraMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	switch k {
	case "1":
		return MenuReturn{
			NextFunction: "perrralotto",
		}
	case "2":
		return MenuReturn{
			NextFunction: "drogslanglotto",
		}
	case "3":
		return MenuReturn{
			NextFunction: "ugandan",
		}
	default:
		return MenuReturn{
			NextFunction: menu.Caller,
		}
	}
}
func (m *ExtraMenu) InputLength() int {
	return 1
}

func (m *ExtraMenu) Name() string {
	return "extramenu"
}

func (m *ExtraMenu) Prefix(c *Controller) {
	message := "Välkommen till extra menyn, för perrra, tryck ett, för drog slang, tryck två, ugandan, tryck tre, för att gå tillbaka, tryck 0"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
