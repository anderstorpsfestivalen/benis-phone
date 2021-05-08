package controller

import (
	"fmt"
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"
)

type DrogSlangLotto struct{}

func (m *DrogSlangLotto) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case key := <-keychan:
			if key == "0" {
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			} else {

				rand.Seed(time.Now().UnixNano())
				min := 1
				max := 16
				number := rand.Intn((max - min + 1) + min)
				fmt.Printf("rand nr is %d\n", number)
				message := ""

				if number == 1 {
					message = "Marijuana. Grönt. W. Weed. Gräs. Skunk. Dunder. Mary Jane. Marie och Ann. Indian. Blue Cheese. Pollen."
				}
				if number == 2 {
					message = "Hasch. Brunt. Afghan. Zutt. Töjj. Blaze. Fett. Gås. B."
				}
				if number == 3 {
					message = "Anfetamin. Speed. Tjack. Sho. Tjosan. Ful. Klet. Vaket. Affe. Skor. Billigt. Ice."
				}
				if number == 4 {
					message = "Kokain. Ladd. Jayo. Kola. Pulver. Snö. Schnejjf. Stenar."
				}
				if number == 5 {
					message = "LSD. Syra. Lucy. Sås. Lappar. Trippar."
				}
				if number == 6 {
					message = "Extacy. M,D,M,A. X,T,C. Eva. E. Erik. Molly. Nappar. Glad."
				}
				if number == 7 {
					message = "Subutex. Båt. Fjärdis. Sub. Åtta."
				}
				if number == 8 {
					message = "Tramadol. Lean. Tram."
				}
				if number == 9 {
					message = "Xanor. Stavar. Blåbär. Blå. Pix."
				}
				if number == 10 {
					message = "Benzo. Habbar. Bitches. Flödder."
				}
				if number == 11 {
					message = "Heroin. Häst. Horse. Jonk. Dop. Frukt. Dope. H. Black tar. Smack."
				}
				if number == 12 {
					message = "Spliff. Joint. Blandning av cannabis och tokab."
				}
				if number == 13 {
					message = "Kasse. Ett Kilo."
				}
				if number == 14 {
					message = "Hegge. Ett Hekto."
				}
				if number == 15 {
					message = "Hasch. Brunt. Afghan. Zutt. Töjj. Blaze. Fett. Gås. B."
				}
				if number == 16 {
					message = "Anfetamin. Speed. Tjack. Sho. Tjosan. Ful. Klet. Vaket. Affe. Skor. Billigt. Ice."
				}

				ttsData, err := c.Polly.TTS(message, "Astrid")
				if err != nil {
					return MenuReturn{
						Error:        err,
						NextFunction: "error",
					}
				}
				go c.Audio.PlayMP3FromStream(ttsData)
			}
		}
	}
}
func (m *DrogSlangLotto) InputLength() int {
	return 0
}

func (m *DrogSlangLotto) Name() string {
	return "syralotto"
}

func (m *DrogSlangLotto) Prefix(c *Controller) {
	message := "Har du koll på drogslangen? TRYCK ETT till FYRKANT, NOLL FÖR ATT GÅ TILLBAKA"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
