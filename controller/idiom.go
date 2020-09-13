package controller

import (
	"io/ioutil"
	"math/rand"
	"strings"
	"time"
)

type Idiom struct {
}

func (m *Idiom) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	data, err := ioutil.ReadFile("files/idiom.txt")
	if err != nil {
		panic(err)
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
			NextFunction: "mainmenu",
		}
	}
	c.Audio.PlayMP3FromStream(ttsData)

	return MenuReturn{
		NextFunction: menu.Caller,
	}

}

func (m *Idiom) InputLength() int {
	return 0
}

func (m *Idiom) Name() string {
	return "idiom"
}
