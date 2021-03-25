package controller

import "fmt"

type Announce struct{}

func (m *Announce) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	// Clear all ongoing audio preparing for incoming announcement
	fmt.Println("Clearing audio")
	c.Audio.Clear()

	keychan := c.Phone.GetKeyChannel()
	for {
		select {
		case key := <-keychan:
			if key == "1" {
				fmt.Println("Returning to main menu")
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			} else {
				fmt.Println("Returning to main menu")
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			}
		}
	}
}
func (m *Announce) InputLength() int {
	return 0
}

func (m *Announce) Name() string {
	return "announce"
}

func (m *Announce) Prefix(c *Controller) {
}
