package controller

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gitlab.com/anderstorpsfestivalen/benis-phone/audio"
	"gitlab.com/anderstorpsfestivalen/benis-phone/phone"
	"gitlab.com/anderstorpsfestivalen/benis-phone/polly"
)

var MenuOptions = map[string]MenuOption{
	"mainmenu":       &MainMenu{},
	"announce":       &Announce{},
	"systemet":       &Systemet{},
	"trainmenu":      &TrainMenu{},
	"barclosingmenu": &BarClosingMenu{},
}

type Controller struct {
	Phone phone.FlowPhone
	Audio *audio.Audio
	Polly polly.Polly
	Where string
	Menu  MenuReturn
}

func New(ph phone.FlowPhone, audio *audio.Audio, polly polly.Polly) Controller {
	return Controller{
		Phone: ph,
		Audio: audio,
		Polly: polly,
		Where: "mainmenu",
		Menu:  MenuReturn{},
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
					c.Audio.PlayFromFile("files/etype.mp3")
				} else {
					hookstate = false
					c.Audio.Clear()
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
					keys += key
					if il == 0 {
						log.WithFields(log.Fields{
							"Function":     c.Where,
							"Input Length": il,
						}).Info("Entering function")
						c.TriggerFunction(keys)
						keys = ""
					} else {
						if len(keys) == il {
							log.WithFields(log.Fields{
								"Function":     c.Where,
								"Input Length": il,
							}).Info("Entering function")
							c.TriggerFunction(keys)
							keys = ""
						}
					}
				}
			default:
				time.Sleep(time.Millisecond * 2)
			}
		}
	}()

}

func (c *Controller) TriggerFunction(keys string) {
	c.Audio.Clear()
	res := MenuOptions[c.Where].Run(c, keys, c.Menu)
	res.Caller = MenuOptions[c.Where].Name()
	c.Menu = res
	c.Where = res.NextFunction

	if MenuOptions[c.Where].InputLength() == 0 {
		c.TriggerFunction("")
	}
}
