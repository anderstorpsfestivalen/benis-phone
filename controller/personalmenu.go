package controller

import (
	log "github.com/sirupsen/logrus"
)

type PersonalMenu struct{}

func (m *PersonalMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	switch k {
	case "1":
		return MenuReturn{
			NextFunction: "balance",
		}
	case "2":
		return MenuReturn{
			NextFunction: "promille",
		}
	case "3":
		return MenuReturn{
			NextFunction: "fulolpoints",
		}
	default:
		return MenuReturn{
			NextFunction: menu.Caller,
		}
	}
}
func (m *PersonalMenu) InputLength() int {
	return 1
}

func (m *PersonalMenu) Name() string {
	return "personalmenu"
}

func (m *PersonalMenu) Prefix(c *Controller) {
	message := "Välkommen till din personliga service meny, för saldo, tryck ett, för promille, tryck två, för fulöls poäng, tryck tre, för att gå tillbaka, tryck 0"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
