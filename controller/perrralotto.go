package controller

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

type PerraLotto struct{}

func (m *PerraLotto) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case key := <-keychan:
			if key == "1" {
				rand.Seed(time.Now().UnixNano())
				min := 1
				max := 29
				number := rand.Intn((max - min + 1) + min)
				filename := "files/perrra/numbers/" + strconv.Itoa(number) + ".ogg"
				fmt.Println(filename)
				go c.Audio.PlayFromFile(filename)
			} else if key == "2" {
				fmt.Println("pressed 2")
				go c.Audio.PlayFromFile("files/perrra/are-bengt.ogg")
			} else if key == "3" {
				fmt.Println("pressed 3")
				go c.Audio.PlayFromFile("files/perrra/avinstallera-win95.ogg")
			} else if key == "4" {
				fmt.Println("pressed 4")
				go c.Audio.PlayFromFile("files/perrra/det-var-inte-bra.ogg")
			} else if key == "5" {
				fmt.Println("pressed 5")
				go c.Audio.PlayFromFile("files/perrra/en-warez-dator.ogg")
			} else if key == "6" {
				fmt.Println("pressed 6")
				go c.Audio.PlayFromFile("files/perrra/fixa-lite-skit.ogg")
			} else if key == "7" {
				fmt.Println("pressed 7")
				go c.Audio.PlayFromFile("files/perrra/fixa-lite-warez.ogg")
			} else if key == "8" {
				fmt.Println("pressed 8")
				go c.Audio.PlayFromFile("files/perrra/hackare-va.ogg")
			} else if key == "9" {
				fmt.Println("pressed 9")
				go c.Audio.PlayFromFile("files/perrra/hur-ar-det-med-mircwaret.ogg")
			} else if key == "*" {
				fmt.Println("pressed *")
				go c.Audio.PlayFromFile("files/perrra/knarket.ogg")
			} else if key == "#" {
				fmt.Println("pressed #")
				go c.Audio.PlayFromFile("files/perrra/pirat-version.ogg")
			} else {
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
