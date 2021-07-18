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

	sub := c.Subscribe(m.Name())
	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case <-sub.Cancel:
			c.Unsubscribe(m.Name())
			return MenuReturn{
				NextFunction: menu.Caller,
			}
		case key := <-keychan:
			switch key {
			case "1":
				files, err := ioutil.ReadDir("./files/ugandan/")
				if err != nil {
					log.Fatal(err)
				}
				rand.Seed(time.Now().UnixNano())
				number := rand.Intn(len(files) - 1)
				filename := "files/ugandan/" + files[number].Name()
				fmt.Println(filename)
				go c.Audio.PlayFromFile(filename)
			case "2":
				fmt.Println("pressed 2")
				go c.Audio.PlayFromFile("files/ugandan/COMMANDO-COMMANDO.ogg")
			case "3":
				fmt.Println("pressed 3")
				go c.Audio.PlayFromFile("files/ugandan/Commando-1.ogg")
			case "4":
				fmt.Println("pressed 4")
				go c.Audio.PlayFromFile("files/ugandan/Gwe-Gwe-Gwe.ogg")
			case "5":
				fmt.Println("pressed 5")
				go c.Audio.PlayFromFile("files/ugandan/One-hell-of-a-movie.ogg")
			case "6":
				fmt.Println("pressed 6")
				go c.Audio.PlayFromFile("files/ugandan/WHAT-THE-FU.ogg")
			case "7":
				fmt.Println("pressed 7")
				go c.Audio.PlayFromFile("files/ugandan/Tough-Commando-on-da-Mission.ogg")
			case "8":
				fmt.Println("pressed 8")
				go c.Audio.PlayFromFile("files/ugandan/UGAANDA.ogg")
			case "9":
				fmt.Println("pressed 9")
				go c.Audio.PlayFromFile("files/ugandan/HELLO-2.ogg")
			case "*":
				fmt.Println("pressed *")
				go c.Audio.PlayFromFile("files/ugandan/SUPA-MAFIA-ON-THE-RUN.ogg")
			case "#":
				fmt.Println("pressed #")
				go c.Audio.PlayFromFile("files/ugandan/Warrior.ogg")
			default:
				return MenuReturn{
					NextFunction: menu.Caller,
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
