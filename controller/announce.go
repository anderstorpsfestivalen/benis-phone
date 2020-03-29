package controller

import "fmt"

type Announce struct{}

func (m *Announce) Run(c *Controller, k string) MenuReturn {

	if k == "121212" {
		fmt.Println("ANNOUNCE")
	} else {
		fmt.Println("WRONG PASSWORD")
	}

	return MenuReturn{
		NextAction:   "LUL",
		NextFunction: "mainmenu",
	}
}
func (m *Announce) InputLength() int {
	return 6
}
