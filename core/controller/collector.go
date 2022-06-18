package controller

import (
	"fmt"
	"strings"

	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
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
	return len(c.buffer) >= c.MaxLength
}

func (c *Collector) GetBuffer() string {
	return strings.Join(c.buffer, "")
}

func (c *Collector) Finish(ctrl *Controller) error {
	buf := c.GetBuffer()
	if c.service != nil {
		return ctrl.runService(*c.service, &buf)
	}

	return fmt.Errorf("collector could not dispatch")
}
