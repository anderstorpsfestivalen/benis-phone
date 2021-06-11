package controller

import (
	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type Idiom struct {
}

func (m *Idiom) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case key := <-keychan:
			if key == "0" {
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			} else {

				data, err := ioutil.ReadFile("files/idiom.txt")
				if err != nil {
					return MenuReturn{
						Error:        err,
						NextFunction: "error",
					}
				}

				var lines []string
				lines = strings.Split(string(data), "\n")

				s1 := rand.NewSource(time.Now().UnixNano())
				r1 := rand.New(s1)
				randomIndex := r1.Intn(len(lines))
				//fmt.Println(randomIndex)

				message := lines[randomIndex]
				//fmt.Println(message)

				ttsData, err := c.Polly.TTS(message, "Astrid")
				if err != nil {
					return MenuReturn{
						Error:        err,
						NextFunction: "error",
					}
				}
				c.Audio.PlayMP3FromStream(ttsData)

			}
		}
	}

}

func (m *Idiom) InputLength() int {
	return 0
}

func (m *Idiom) Name() string {
	return "idiom"
}

func (m *Idiom) Prefix(c *Controller) {
	message := "TRYCK, ETT, till FYRKANT FÖR LITE SKÖNA IDIOM, NOLL FÖR ATT GÅ TILLBAKA"
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
