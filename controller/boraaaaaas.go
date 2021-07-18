package controller

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"
)

type Boraaaaaas struct {
}

func (m *Boraaaaaas) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

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
				files, err := ioutil.ReadDir("./files/chatten/")
				if err != nil {
					log.Fatal(err)
				}
				rand.Seed(time.Now().UnixNano())
				number := rand.Intn(len(files))
				filename := "files/chatten/" + files[number].Name()
				fmt.Println(filename)
				go c.Audio.PlayFromFile(filename)
			case "2":
				fmt.Println("pressed 2")
				go c.Audio.PlayFromFile("files/boraaaaaas.ogg")
			case "3":
				fmt.Println("pressed 3")
				go c.Audio.PlayFromFile("files/chatten/booooooooooooras.ogg")
			case "4":
				fmt.Println("pressed 4")
				go c.Audio.PlayFromFile("files/chatten/festen-ar-imorgon.ogg")
			case "5":
				fmt.Println("pressed 5")
				go c.Audio.PlayFromFile("files/chatten/rom-of-rolf.ogg")
			case "6":
				fmt.Println("pressed 6")
				go c.Audio.PlayFromFile("files/chatten/pastiiiiissss.ogg")
			case "7":
				fmt.Println("pressed 7")
				go c.Audio.PlayFromFile("files/chatten/luktar-te-qila.ogg")
			case "8":
				fmt.Println("pressed 8")
				go c.Audio.PlayFromFile("files/chatten/if-its-up-its-up.ogg")
			case "9":
				fmt.Println("pressed 9")
				go c.Audio.PlayFromFile("files/chatten/jaja-sager-vi.ogg")
			case "*":
				fmt.Println("pressed *")
				go c.Audio.PlayFromFile("files/chatten/johanna-toalett.ogg")
			case "#":
				fmt.Println("pressed #")
				go c.Audio.PlayFromFile("files/chatten/halla-klockan-8.ogg")
			default:
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			}
		}
	}
}

func (m *Boraaaaaas) InputLength() int {
	return 0
}

func (m *Boraaaaaas) Name() string {
	return "boraaaaaas"
}

func (m *Boraaaaaas) Prefix(c *Controller) {
	message := "TRYCK, ETT, till FYRKANT, NOLL FÖR ATT GÅ TILLBAKA"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
