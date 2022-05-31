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
	"gitlab.com/anderstorpsfestivalen/benis-phone/services"
)

type Controller struct {
	Phone      phone.FlowPhone
	Audio      *audio.Audio
	Recorder   audio.Recorder
	Polly      polly.Polly
	Definition functions.Definition

	Callstack []string

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
				log.Trace("Controller got key:", key)
				if c.HookState {
					log.Trace("Controller acts on key", key, ". Controller is at:", "")
					c.Audio.Clear()

					c.handleKey(key)

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

	pr, err := c.getCurrent().Prefix.GetPlayable()
	if err != nil {
		return err
	}

	err = pr.Play(c.Audio, c.Polly)
	if err != nil {
		return err
	}
	return nil
}

// This is the actual control flow
func (c *Controller) handleKey(key string) {
	// Exit, pop one from the callstack
	if key == "0" {
		if c.getCurrent().ClearCallstack {
			c.clearCallstack()
		} else {
			c.exitFunction()
		}
		return
	}

	// Resolve action
	action, err := c.getCurrent().ResolveAction(key)
	if err != nil {
		if err.Error() == "could not find key" {
			log.Trace("%v %v %v", key, *c.getCurrent(), err.Error())
			return
		} else {
			c.checkError(err)
		}
	}

	actionType, err := action.Type()
	if err != nil {
		c.checkError(err)
		return
	}

	switch actionType {
	case "fn":
		c.checkError(c.enterFunction(action.Dst))
	case "file":
		c.play(action.File)
	case "randomfile":
		c.play(action.RandomFile)
	case "srv":
		err := c.runService(action.Service)
		c.checkError(err)
	}
}

// Appends to the callstack if function exists and tries to schedule prefix
func (c *Controller) enterFunction(dst string) error {
	if newFn, ok := c.Definition.Functions[dst]; ok {
		c.Callstack = append(c.Callstack, newFn.Name)
		c.prefixSignal <- true
		return nil
	}
	return fmt.Errorf("could not find dst function: %v", dst)
}

// Removes from the callstack
func (c *Controller) exitFunction() {
	if len(c.Callstack) > 1 {
		c.Callstack = c.Callstack[:len(c.Callstack)-1]
		c.prefixSignal <- true
	}
}

// Reset Callstack
func (c *Controller) clearCallstack() {
	c.Callstack = c.Callstack[:0]
	c.Callstack = append(c.Callstack, c.Definition.General.Entrypoint)
	c.prefixSignal <- true
}

// Run service
func (c *Controller) runService(srv functions.Service) error {
	data, err := services.ServiceRegistry[srv.Destination].Get("", srv.Template, srv.Arguments)
	if err != nil {
		return err
	}

	t := c.Definition.StandardTTS(data)
	if srv.TTS != (functions.TTS{}) {
		t = srv.TTS
		t.Message = data
	}

	c.play(t)

	return nil
}

// Gets the most recent added function from the callstack
func (c *Controller) getCurrent() *functions.Fn {
	current := c.Callstack[len(c.Callstack)-1]
	return c.Definition.Functions[current]
}

func (c *Controller) liftHook() {
	c.hs.Lock()
	c.HookState = true
	c.Audio.Clear()
	c.Callstack = append(c.Callstack, c.Definition.General.Entrypoint)
	c.prefixSignal <- true

	log.Info("Hook is lifted")
	c.hs.Unlock()
}

func (c *Controller) slamHook() {
	c.hs.Lock()
	c.HookState = false
	c.Audio.Clear()

	c.Callstack = c.Callstack[:0]

	log.Info("Hook is slammed")
	c.hs.Unlock()
}

func (c *Controller) play(pl functions.PlayGenerator) {
	p := functions.CreatePlayable(pl)
	log.Trace("Playing %v", p)
	p.Play(c.Audio, c.Polly)
}

func (c *Controller) checkError(e error) {
	if e != nil {
		// Log the error
		log.Error(e)

		// Try to read out the error with TTS
		c.play(c.Definition.EnglishTTS(e.Error()))

	}
}
