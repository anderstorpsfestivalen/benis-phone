package controller

type MenuOption interface {
	Run(c *Controller, key string) MenuReturn
	InputLength() int
	Name() string
}

type MenuReturn struct {
	NextAction   string
	NextFunction string
}
