package controller

import (
	"sync"
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

func (c *Controller) Start(wg *sync.WaitGroup) {

	var keys string
	var hookstate bool

	hookchan := c.Phone.GetHookChannel()
	keychan := c.Phone.GetKeyChannel()

	go func() {
		for {
			select {
			case hook := <-hookchan:
				if hook {
					hookstate = true
					log.Info("Hook is lifted")
					c.Mpd.Add("default.mp3")
					c.Mpd.PlayBlocking()
				} else {
					hookstate = false
					c.Mpd.Clear()
					c.Where = "mainmenu"
					log.Info("Hook is slammed")
				}
			default:
				time.Sleep(time.Millisecond * 2)
			}
		}
	}()

	go func() {
		for {
			select {
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
	}()

}

func (c *Controller) TriggerFunction(keys string) {
	res := MenuOptions[c.Where].Run(c, keys)
	c.Where = res.NextFunction
}
