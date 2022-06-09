package drogslang

import (
	"bytes"
	"html/template"
	"math/rand"
	"time"
)

type Drogslang struct{}

func (f *Drogslang) Get(input string, tmpl string, arguments map[string]string) (string, error) {
	ttmpl, err := template.New("drogslang").Parse(tmpl)
	if err != nil {
		return "", err
	}

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

	type DS struct {
		Slang string
	}

	ds := DS{
		Slang: message,
	}

	buf := new(bytes.Buffer)
	err = ttmpl.Execute(buf, ds)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (t *Drogslang) MaxInputLength() int {
	return 0
}
