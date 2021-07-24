package controller

import (
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"
)

type DrogSlangLotto struct{}

func (m *DrogSlangLotto) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	drogsp := "Har du koll på drogslangen?"
	ttsData, err := c.Polly.TTS(drogsp, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)

	rand.Seed(time.Now().UnixNano())
	min := 1
	max := 16
	number := rand.Intn((max - min + 1) + min)

	message := ""
	switch number {

	case 1:
		message = "Marijuana. Grönt. W. Weed. Gräs. Skunk. Dunder. Mary Jane. Marie och Ann. Indian. Blue Cheese. Pollen."
	case 2:
		message = "Hasch. Brunt. Afghan. Zutt. Töjj. Blaze. Fett. Gås. B."
	case 3:
		message = "Anfetamin. Speed. Tjack. Sho. Tjosan. Ful. Klet. Vaket. Affe. Skor. Billigt. Ice."
	case 4:
		message = "Kokain. Ladd. Jayo. Kola. Pulver. Snö. Schnejjf. Stenar."
	case 5:
		message = "LSD. Syra. Lucy. Sås. Lappar. Trippar."
	case 6:
		message = "Extacy. M,D,M,A. X,T,C. Eva. E. Erik. Molly. Nappar. Glad."
	case 7:
		message = "Subutex. Båt. Fjärdis. Sub. Åtta."
	case 8:
		message = "Tramadol. Lean. Tram."
	case 9:
		message = "Xanor. Stavar. Blåbär. Blå. Pix."
	case 10:
		message = "Benzo. Habbar. Bitches. Flödder."
	case 11:
		message = "Heroin. Häst. Horse. Jonk. Dop. Frukt. Dope. H. Black tar. Smack."
	case 12:
		message = "Spliff. Joint. Blandning av cannabis och tobak."
	case 13:
		message = "Kasse. Ett Kilo."
	case 14:
		message = "Hegge. Ett Hekto."
	case 15:
		message = "Hasch. Brunt. Afghan. Zutt. Töjj. Blaze. Fett. Gås. B."
	case 16:
		message = "Anfetamin. Speed. Tjack. Sho. Tjosan. Ful. Klet. Vaket. Affe. Skor. Billigt. Ice."
	}
	ttsData2, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		return MenuReturn{
			Error:        err,
			NextFunction: "error",
		}
	}
	c.Audio.PlayMP3FromStream(ttsData2)

	return MenuReturn{
		NextFunction: "mainmenu",
	}

}

func (m *DrogSlangLotto) InputLength() int {
	return 0
}

func (m *DrogSlangLotto) Name() string {
	return "syralotto"
}

func (m *DrogSlangLotto) Prefix(c *Controller) {

}
