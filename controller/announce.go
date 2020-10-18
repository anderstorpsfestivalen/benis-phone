package controller

type Announce struct{}

func (m *Announce) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	// Clear all ongoing audio preparing for incoming announcement
	c.Audio.Clear()

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
