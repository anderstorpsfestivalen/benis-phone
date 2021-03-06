package controller

import (
	"fmt"
)

type MainMenu struct {
}

func (m *MainMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	c.Audio.Clear()
	fmt.Println("Main Menu Recieve: " + k)
	switch k {
	case "1":
		return MenuReturn{
			NextFunction: "personalmenu",
		}
	case "2":
		return MenuReturn{
			NextFunction: "barmenu",
		}
	case "3":
		return MenuReturn{
			NextFunction: "trainmenu",
		}
	case "4":
		return MenuReturn{
			NextFunction: "systemetmenu",
		}
	case "5":
		return MenuReturn{
			NextFunction: "flacornot",
		}
	case "6":
		return MenuReturn{
			NextFunction: "idiom",
		}
	case "7":
		return MenuReturn{
			NextFunction: "boraaaaaas",
		}
	case "8":
		return MenuReturn{
			NextFunction: "trainmenu",
		}
	case "9":
		return MenuReturn{
			NextFunction: "extramenu",
		}
	case "*":
		return MenuReturn{
			NextFunction: "recordmessage",
		}
	case "0":
		return MenuReturn{
			NextFunction: "announce",
		}
	case "#":
		return MenuReturn{
			NextFunction: "queue",
		}
		// # does not work with virtual keyboard, adding temp function to fault trace
	}
	return MenuReturn{
		NextFunction: "mainmenu",
	}
}

func (m *MainMenu) InputLength() int {
	return 1
}

func (m *MainMenu) Name() string {
	return "mainmenu"
}

func (m *MainMenu) Prefix(c *Controller) {
	c.Audio.Clear()
	c.Audio.PlayFromFile("files/atp-intro.mp3")
}
