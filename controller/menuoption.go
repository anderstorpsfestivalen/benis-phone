package controller

type MenuOption interface {
	Run(c *Controller, key string, menu MenuReturn) MenuReturn
	InputLength() int
	Name() string
}

type MenuReturn struct {
	Caller       string
	NextFunction string
}
