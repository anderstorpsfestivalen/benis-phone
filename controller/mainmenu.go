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
			NextFunction: "syralotto",
		}
	case "9":
		return MenuReturn{
			NextFunction: "extramenu",
		}
	case "*":
		return MenuReturn{
			NextFunction: "recordmessage",
			// to be leave a message function - test this
		}
	case "0":
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
