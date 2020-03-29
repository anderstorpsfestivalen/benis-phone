package controller

import (
	"time"

	log "github.com/sirupsen/logrus"
	"gitlab.com/anderstorpsfestivalen/benis-phone/mpd"
	"gitlab.com/anderstorpsfestivalen/benis-phone/phone"
)

var MenuOptions = map[string]MenuOption{
	"mainmenu": &MainMenu{},
	"announce": &Announce{},
}

type Controller struct {
	Phone phone.FlowPhone
	Mpd   mpd.MpdClient
	Where string
}

func New(ph phone.FlowPhone, mpd mpd.MpdClient) Controller {
	return Controller{
		Phone: ph,
		Mpd:   mpd,
		Where: "mainmenu",
	}
}

func (c *Controller) Start() {

	var keys string
	var hookstate bool

	hookchan := c.Phone.GetHookChannel()
	keychan := c.Phone.GetKeyChannel()

	log.Info("Starting Main Loop")
	for {
		select {
		case hook := <-hookchan:
			if hook {
				hookstate = true
				log.Info("Hook is lifted")
			} else {
				hookstate = false
				c.Mpd.Clear()
				c.Where = "mainmenu"
				log.Info("Hook is slammed")
			}
		case key := <-keychan:
			if hookstate {
				il := MenuOptions[c.Where].InputLength()
				log.WithFields(log.Fields{
					"Function":     c.Where,
					"Input Length": il,
				}).Info("Entering function")
				keys += key
				if len(keys) == il {
					c.TriggerFunction(keys)
					keys = ""
				}
			}
		default:
			time.Sleep(time.Millisecond * 2)
		}
	}
}

func (c *Controller) TriggerFunction(keys string) {
	res := MenuOptions[c.Where].Run(c, keys)
	c.Where = res.NextFunction
}
