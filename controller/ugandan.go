package controller

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"
)

type Ugandan struct{}

func (m *Ugandan) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case key := <-keychan:
			if key == "1" {
				files, err := ioutil.ReadDir("./files/ugandan/")
				if err != nil {
					log.Fatal(err)
				}
				rand.Seed(time.Now().UnixNano())
				number := rand.Intn(len(files) - 1)
				filename := "files/ugandan/" + files[number].Name()
				fmt.Println(filename)
				go c.Audio.PlayFromFile(filename)
			} else if key == "2" {
				fmt.Println("pressed 2")
				go c.Audio.PlayFromFile("files/ugandan/COMMANDO-COMMANDO.ogg")
			} else if key == "3" {
				fmt.Println("pressed 3")
				go c.Audio.PlayFromFile("files/ugandan/Commando-1.ogg")
			} else if key == "4" {
				fmt.Println("pressed 4")
				go c.Audio.PlayFromFile("files/ugandan/Gwe-Gwe-Gwe.ogg")
			} else if key == "5" {
				fmt.Println("pressed 5")
				go c.Audio.PlayFromFile("files/ugandan/One-hell-of-a-movie.ogg")
			} else if key == "6" {
				fmt.Println("pressed 6")
				go c.Audio.PlayFromFile("files/ugandan/WHAT-THE-FU.ogg")
			} else if key == "7" {
				fmt.Println("pressed 7")
				go c.Audio.PlayFromFile("files/ugandan/Tough-Commando-on-da-Mission.ogg")
			} else if key == "8" {
				fmt.Println("pressed 8")
				go c.Audio.PlayFromFile("files/ugandan/UGAANDA.ogg")
			} else if key == "9" {
				fmt.Println("pressed 9")
				go c.Audio.PlayFromFile("files/ugandan/HELLO-2.ogg")
			} else if key == "*" {
				fmt.Println("pressed *")
				go c.Audio.PlayFromFile("files/ugandan/SUPA-MAFIA-ON-THE-RUN.ogg")
			} else if key == "#" {
				fmt.Println("pressed #")
				go c.Audio.PlayFromFile("files/ugandan/Warrior.ogg")
			} else {
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			}
		}
	}
}

func (m *Ugandan) InputLength() int {
	return 0
}

func (m *Ugandan) Name() string {
	return "ugandan"
}

func (m *Ugandan) Prefix(c *Controller) {
	message := "TRYCK, ETT, till FYRKANT, NOLL FÖR ATT GÅ TILLBAKA"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
