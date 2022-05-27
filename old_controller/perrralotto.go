package controller

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"
)

type PerraLotto struct{}

func (m *PerraLotto) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	sub := c.Subscribe(m.Name())
	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case <-sub.Cancel:
			c.Unsubscribe(m.Name())
			return MenuReturn{
				NextFunction: "mainmenu",
			}
		case key := <-keychan:
			switch key {
			case "1":
				files, err := ioutil.ReadDir("./files/perrra/")
				if err != nil {
					log.Fatal(err)
				}
				rand.Seed(time.Now().UnixNano())
				number := rand.Intn(len(files) - 1)
				filename := "files/perrra/" + files[number].Name()
				fmt.Println(filename)
				go c.Audio.PlayFromFile(filename)
			case "2":
				fmt.Println("pressed 2")
				go c.Audio.PlayFromFile("files/perrra/are-bengt.ogg")
			case "3":
				fmt.Println("pressed 3")
				go c.Audio.PlayFromFile("files/perrra/avinstallera-win95.ogg")
			case "4":
				fmt.Println("pressed 4")
				go c.Audio.PlayFromFile("files/perrra/det-var-inte-bra.ogg")
			case "5":
				fmt.Println("pressed 5")
				go c.Audio.PlayFromFile("files/perrra/en-warez-dator.ogg")
			case "6":
				fmt.Println("pressed 6")
				go c.Audio.PlayFromFile("files/perrra/fixa-lite-skit.ogg")
			case "7":
				fmt.Println("pressed 7")
				go c.Audio.PlayFromFile("files/perrra/fixa-lite-warez.ogg")
			case "8":
				fmt.Println("pressed 8")
				go c.Audio.PlayFromFile("files/perrra/hackare-va.ogg")
			case "9":
				fmt.Println("pressed 9")
				go c.Audio.PlayFromFile("files/perrra/hur-ar-det-med-mircwaret.ogg")
			case "*":
				fmt.Println("pressed *")
				go c.Audio.PlayFromFile("files/perrra/knarket.ogg")
			case "#":
				fmt.Println("pressed #")
				go c.Audio.PlayFromFile("files/perrra/pirat-version.ogg")
			default:
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			}
		}
	}
}
func (m *PerraLotto) InputLength() int {
	return 0
}

func (m *PerraLotto) Name() string {
	return "perralotto"
}

func (m *PerraLotto) Prefix(c *Controller) {
	message := "TRYCK ETT FÖR RANDOM, TVÅ till FYRKANT FÖR FASTA ALTERNATIV, NOLL FÖR ATT GÅ TILLBAKA"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
