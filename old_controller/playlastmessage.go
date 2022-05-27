package controller

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"
)

type PlayLastMessage struct {
}

func (m *PlayLastMessage) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	dir := "temp/message"
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var modTime time.Time
	var names []string
	for _, f := range files {
		if f.Mode().IsRegular() {
			if !f.ModTime().Before(modTime) {
				if f.ModTime().After(modTime) {
					modTime = f.ModTime()
					names = names[:0]
				}
				names = append(names, f.Name())
			}
		}
	}
	if len(names) > 0 {
		filename := dir + "/" + names[0]
		//fmt.Println(filename)
		go c.Audio.PlayFromFile(filename)
	}
	return MenuReturn{
		NextFunction: menu.Caller,
	}
}
func (m *PlayLastMessage) InputLength() int {
	return 0
}

func (m *PlayLastMessage) Name() string {
	return "playlastmessage"
}

func (m *PlayLastMessage) Prefix(c *Controller) {
}
