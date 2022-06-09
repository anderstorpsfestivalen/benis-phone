package controller

import (
	"fmt"
	"strings"

	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/functions"
)

type Collector struct {
	MaxLength int
	buffer    []string

	service *functions.Service
}

func CreateServiceCollector(ml int, srv *functions.Service) *Collector {
	return &Collector{
		MaxLength: ml,

		service: srv,
	}
}

func (c *Collector) CollectKey(key string) bool {
	c.buffer = append(c.buffer, key)

	fmt.Println(c.buffer)
	fmt.Println(len(c.buffer), c.MaxLength)

	return len(c.buffer) >= c.MaxLength
}

func (c *Collector) GetBuffer() string {
	return strings.Join(c.buffer, "")
}

func (c *Collector) Finish(ctrl *Controller) {
	buf := c.GetBuffer()
	if c.service != nil {
		ctrl.runService(*c.service, &buf)
	}
}
