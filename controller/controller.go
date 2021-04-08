package controller

import (
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/audio"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/phone"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/polly"
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/systemet"
)

var MenuOptions = map[string]MenuOption{
	"mainmenu":          &MainMenu{},
	"announce":          &Announce{},
	"systemetarboga":    &SystemetArboga{},
	"trainmenu":         &TrainMenu{},
	"barclosingmenu":    &BarClosingMenu{},
	"syralotto":         &SyraLotto{},
	"currentmenu":       &CurrentMenu{},
	"flacornot":         &FlacOrNotMenu{},
	"idiom":             &Idiom{},
	"balance":           &Balance{},
	"barindex":          &BarIndex{},
	"systemetpid":       &SystemetPid{},
	"recordmessage":     &RecordMessage{},
	"playlastmessage":   &PlayLastMessage{},
	"playrandommessage": &PlayRandomMessage{},
	"error":             &Err{},
	"barmenu":           &BarMenu{},
	"personalmenu":      &PersonalMenu{},
	"systemetmenu":      &SystemetMenu{},
	"promille":          &Promille{},
	"fulolpoints":       &FulolPoints{},
}

type Controller struct {
	Phone       phone.FlowPhone
	Audio       *audio.Audio
	Recorder    audio.Recorder
	Polly       polly.Polly
	SystemetAPI systemet.SystemetV2
	Where       string
	Menu        MenuReturn

	Settings ControllerSettings
}

type ControllerSettings struct {
	HiddenPlayback bool
}

func New(ph phone.FlowPhone, audio *audio.Audio, rec audio.Recorder, polly polly.Polly, sapi systemet.SystemetV2, settings ControllerSettings) Controller {
	return Controller{
		Phone:       ph,
		Audio:       audio,
		Recorder:    rec,
		Polly:       polly,
		SystemetAPI: sapi,
		Where:       "mainmenu",
		Menu:        MenuReturn{},

		Settings: settings,
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
					tm := time.Now()
					recTime := tm.Format("2006-01-02_15:04:05")
					c.Recorder.Record("random/" + recTime)
					go MenuOptions[c.Where].Prefix(c)
				} else {
					hookstate = false
					c.Audio.Clear()
					c.Recorder.Stop()
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
						if len(keys) == il || key == "#" {

							keys = strings.Replace(keys, "#", "", -1)
							log.WithFields(log.Fields{
								"Function":     c.Where,
								"Input Length": il,
							}).Info("Entering function")
							c.TriggerFunction(keys)
							keys = ""
						}
					}
				}
			}
		}
	}()

}

func (c *Controller) TriggerFunction(keys string) {
	c.Audio.Clear()
	res := MenuOptions[c.Where].Run(c, keys, c.Menu)
	if res.NextFunction == "nil" {
		return
	}
	res.Caller = MenuOptions[c.Where].Name()
	c.Menu = res
	c.Where = res.NextFunction
	go MenuOptions[c.Where].Prefix(c)

	if MenuOptions[c.Where].InputLength() == 0 {
		c.TriggerFunction("")
	}
}
