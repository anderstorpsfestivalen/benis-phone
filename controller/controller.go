package controller

import (
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/audio"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/functions"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/phone"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/polly"
)

type Controller struct {
	Phone      phone.FlowPhone
	Audio      *audio.Audio
	Recorder   audio.Recorder
	Polly      polly.Polly
	Definition functions.Definition

	Current   string
	HookState bool

	hs           sync.Mutex
	prefixSignal chan bool
}

func New(ph phone.FlowPhone, audio *audio.Audio, rec audio.Recorder, polly polly.Polly, def functions.Definition) Controller {
	return Controller{
		Phone:      ph,
		Audio:      audio,
		Recorder:   rec,
		Polly:      polly,
		Definition: def,

		prefixSignal: make(chan bool, 100),
	}
}

func (c *Controller) Start(wg *sync.WaitGroup) {

	c.Current = c.Definition.General.Entrypoint

	//fmt.Println(c.Definition)

	//Setup Hook parsing
	hookchan := c.Phone.GetHookChannel()
	go func() {
		for {
			select {
			case hook := <-hookchan:
				if hook {
					c.liftHook()
				} else {
					c.slamHook()
				}
			default:
				//Debounce
				time.Sleep(time.Millisecond * 2)
			}
		}
	}()

	keychan := c.Phone.GetKeyChannel()
	go func() {
		for {
			select {
			case key := <-keychan:
				fmt.Println("Controller got key:", key)
				if c.HookState {
					fmt.Println("Controller acts on key", key, ". Controller is at:", "")
					// il := MenuOptions[c.Where].InputLength()
					// keys += key

					// if il == 0 || c.Where == "mainmenu" {
					// 	fmt.Println("controller trigger")
					// 	log.WithFields(log.Fields{
					// 		"Function":     c.Where,
					// 		"Input Length": il,
					// 	}).Info("Entering function")
					// 	c.TriggerFunction(keys)
					// 	keys = ""
					// } else {
					// 	fmt.Println("controller waits")
					// 	if len(keys) == il || key == "#" {

					// 		keys = strings.Replace(keys, "#", "", -1)
					// 		log.WithFields(log.Fields{
					// 			"Function":     c.Where,
					// 			"Input Length": il,
					// 		}).Info("Entering function")
					// 		c.TriggerFunction(keys)
					// 		keys = ""
					// 	}
					// }
				}

			//Run prefix
			case <-c.prefixSignal:
				err := c.handlePrefix()
				c.checkError(err)
			}

		}
	}()
}

func (c *Controller) handlePrefix() error {

	pr, err := c.Definition.Functions[c.Current].Prefix.GetPlayable()
	if err != nil {
		return err
	}

	err = pr.Play(c.Audio, c.Polly)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) liftHook() {
	c.hs.Lock()
	c.HookState = true
	c.Audio.Clear()
	log.Info("Hook is lifted")
	c.prefixSignal <- true
	c.hs.Unlock()
}

func (c *Controller) slamHook() {
	c.hs.Lock()
	c.HookState = false
	c.Audio.Clear()
	log.Info("Hook is slammed")
	c.hs.Unlock()
}

func (c *Controller) checkError(e error) {
	if e != nil {
		// Log the error
		log.Error(e)

		// Try to read out the error with TTS
		p := functions.CreatePlayableFromTTS(c.Definition.EnglishTTS(e.Error()))
		p.Play(c.Audio, c.Polly)

	}
}
