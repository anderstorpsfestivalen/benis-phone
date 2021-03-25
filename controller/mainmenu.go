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
			NextFunction: "flacornot",
		}
	case "2":
		return MenuReturn{
			NextFunction: "systemet", // to be weather
		}
	case "3":
		return MenuReturn{
			NextFunction: "barmenu",
		}
	case "4":
		return MenuReturn{
			NextFunction: "", // to be personal services menu
			// saldo, promillekoll
		}
	case "5":
		return MenuReturn{
			NextFunction: "", // to be systemet menu
			// menu with functions arboga i lager gislaved, slå upp produkt id,
		}
	case "6":
		return MenuReturn{
			NextFunction: "idiom",
		}
	case "7":
		return MenuReturn{
			NextFunction: "trainmenu",
		}
	case "8":
		return MenuReturn{
			NextFunction: "syralotto",
		}
	case "9":
		return MenuReturn{
			NextFunction: "playrandommessage",
		}
	case "*":
		return MenuReturn{
			NextFunction: "", // to be leave a message function
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
