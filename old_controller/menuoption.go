package controller

type MenuOption interface {
	Run(c *Controller, key string, menu MenuReturn) MenuReturn
	InputLength() int
	Name() string
	Prefix(c *Controller)
}

type MenuReturn struct {
	Error        error
	Caller       string
	NextFunction string
}
