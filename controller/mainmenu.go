package controller

import (
	"fmt"
)

type MainMenu struct {
}

func (m *MainMenu) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	c.Audio.Clear()
	fmt.Println("RECEIVED: " + k)
	switch k {
	case "1":
		return MenuReturn{
			NextFunction: "balance",
		}

	case "2":
		return MenuReturn{
			NextFunction: "barindex",
		}
	case "3":
		return MenuReturn{
			NextFunction: "systemetpidmenu",
		}
	case "4":
		return MenuReturn{
			NextFunction: "trainmenu",
		}
	case "5":
		return MenuReturn{
			NextFunction: "systemet",
		}
	case "7":
		return MenuReturn{
			NextFunction: "syralotto",
		}
	case "8":
		return MenuReturn{
			NextFunction: "flacornot",
		}
	case "9":
		return MenuReturn{
			NextFunction: "idiom",
		}
	case "R":
		return MenuReturn{
			NextFunction: "announce",
		}
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
	c.Audio.PlayFromFile("files/flocc.ogg")
}
