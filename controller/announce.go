package controller

import "fmt"

type Announce struct{}

func (m *Announce) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	if k == "121212" {
		fmt.Println("ANNOUNCE")
	} else {
		fmt.Println("WRONG PASSWORD")
	}

	return MenuReturn{
		NextFunction: "mainmenu",
	}
}
func (m *Announce) InputLength() int {
	return 6
}

func (m *Announce) Name() string {
	return "announce"
}

func (m *Announce) Prefix(c *Controller) {
}
