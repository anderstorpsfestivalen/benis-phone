package controller

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"
)

type Boraaaaaas struct{}

func (m *Boraaaaaas) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case key := <-keychan:
			if key == "1" {
				files, err := ioutil.ReadDir("./files/chatten/")
				if err != nil {
					log.Fatal(err)
				}
				rand.Seed(time.Now().UnixNano())
				min := 1
				max := 41
				number := rand.Intn(max - min + 1)
				filename := "files/chatten/" + files[number].Name()
				fmt.Println(filename)
				go c.Audio.PlayFromFile(filename)
			} else if key == "2" {
				fmt.Println("pressed 2")
				go c.Audio.PlayFromFile("files/boraaaaaas.ogg")
			} else if key == "3" {
				fmt.Println("pressed 3")
				go c.Audio.PlayFromFile("files/chatten/booooooooooooras.ogg")
			} else if key == "4" {
				fmt.Println("pressed 4")
				go c.Audio.PlayFromFile("files/chatten/festen-ar-imorgon.ogg")
			} else if key == "5" {
				fmt.Println("pressed 5")
				go c.Audio.PlayFromFile("files/chatten/rom-of-rolf.ogg")
			} else if key == "6" {
				fmt.Println("pressed 6")
				go c.Audio.PlayFromFile("files/chatten/pastiiiiissss.ogg")
			} else if key == "7" {
				fmt.Println("pressed 7")
				go c.Audio.PlayFromFile("files/chatten/luktar-te-qila.ogg")
			} else if key == "8" {
				fmt.Println("pressed 8")
				go c.Audio.PlayFromFile("files/chatten/if-its-up-its-up.ogg")
			} else if key == "9" {
				fmt.Println("pressed 9")
				go c.Audio.PlayFromFile("files/chatten/jaja-sager-vi.ogg")
			} else if key == "*" {
				fmt.Println("pressed *")
				go c.Audio.PlayFromFile("files/chatten/johanna-toalett.ogg")
			} else if key == "#" {
				fmt.Println("pressed #")
				go c.Audio.PlayFromFile("files/chatten/halla-klockan-8.ogg")
			} else {
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
