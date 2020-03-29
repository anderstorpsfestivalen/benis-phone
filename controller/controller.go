package controller

import (
	"gitlab.com/anderstorpsfestivalen/benis-phone/dtmf"

	"gitlab.com/anderstorpsfestivalen/benis-phone/mpd"
	"gitlab.com/anderstorpsfestivalen/benis-phone/phone"
)

var MenuOptions = map[string]MenuOption{
	"mainmenu": &MainMenu{},
}

type Controller struct {
	Phone phone.Phone
	Mpd   mpd.MpdClient
	Dtmf  dtmf.Dtmf
	Where string
}

func New(ph phone.Phone, mpd mpd.MpdClient, dtmf dtmf.Dtmf) Controller {
	return Controller{
		Phone: ph,
		Mpd:   mpd,
		Dtmf:  dtmf,
		Where: "mainmenu",
	}
}

func (c *Controller) Start() {

	var keys string

	for {
		c.Where = "mainmenu"
		s := <-c.Phone.HookChannel
		if s {
			select {
			case dtmf_key := <-c.Dtmf.HookKey:
				il := MenuOptions[c.Where].InputLength()
				if len(keys) < il {
					keys += dtmf_key
				} else {
					MenuOptions[c.Where].Run(c, keys)
				}
			}
		} else {
			c.Mpd.Clear()
		}
	}
}
