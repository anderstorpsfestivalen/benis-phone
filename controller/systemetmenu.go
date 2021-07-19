package controller

import (
	log "github.com/sirupsen/logrus"
)

type SystemetMenu struct{}

func (m *SystemetMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	switch k {
	case "1":
		c.Audio.Clear()
		return MenuReturn{
			NextFunction: "systemetarboga",
		}
	case "2":
		c.Audio.Clear()
		return MenuReturn{
			NextFunction: "systemetpid",
		}
	default:
		return MenuReturn{
			NextFunction: menu.Caller,
		}
	}

}

func (m *SystemetMenu) InputLength() int {
	return 1
}

func (m *SystemetMenu) Name() string {
	return "systemetmenu"
}

func (m *SystemetMenu) Prefix(c *Controller) {
	c.Audio.Clear()
	message := "Tryck ett, för antalet arboga 10 komma 2 i lager på systembolaget i gislaved, tryck två, för systembolaget produkt sök, för att återgå, tryck 0"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
