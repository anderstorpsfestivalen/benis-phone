package controller

import (
	"gitlab.com/anderstorpsfestivalen/benis-phone/mpd"
	"gitlab.com/anderstorpsfestivalen/benis-phone/phone"
)

type Controller struct {
	phone phone.Phone
	mpd   mpd.MpdClient
}

func New(ph phone.Phone, mpd mpd.MpdClient) Controller {
	return Controller{
		phone: ph,
		mpd:   mpd,
	}
}

func (c *Controller) Start() {

	for {
		s := <-c.phone.HookChannel
		if s {
			//c.mpd.Add("test.mp3")
			c.mpd.Play()
		} else {
			c.mpd.Clear()
		}
	}
}
