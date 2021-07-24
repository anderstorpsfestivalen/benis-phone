package controller

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"time"
)

type PlayRandomMessage struct {
}

func (m *PlayRandomMessage) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	dir := "temp/message"
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)
	randomIndex := r.Intn(len(files))

	// For debug
	//fmt.Println(randomIndex)
	//fmt.Println(files[randomIndex].Name())

	go c.Audio.PlayFromFile(files[randomIndex].Name())

	return MenuReturn{
		NextFunction: "mainmenu",
	}
}
func (m *PlayRandomMessage) InputLength() int {
	return 0
}

func (m *PlayRandomMessage) Name() string {
	return "PlayRandomMessage"
}

func (m *PlayRandomMessage) Prefix(c *Controller) {
}
