package controller

import "fmt"

type SyraLotto struct{}

func (m *SyraLotto) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case key := <-keychan:
			if key == "1" {
				fmt.Println("pressed 1")
				c.Audio.PlayFromFile("files/lasse-liten/e.ogg")
			} else if key == "2" {
				fmt.Println("pressed 2")
				c.Audio.PlayFromFile("files/lasse-liten/lsd.ogg")
			} else {
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			}
		}
	}
}
func (m *SyraLotto) InputLength() int {
	return 0
}

func (m *SyraLotto) Name() string {
	return "syralotto"
}
