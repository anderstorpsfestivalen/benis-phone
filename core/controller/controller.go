package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
	"github.com/anderstorpsfestivalen/benis-phone/core/phone"
	"github.com/anderstorpsfestivalen/benis-phone/core/polly"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services"
	log "github.com/sirupsen/logrus"
)

type Controller struct {
	Phone      phone.FlowPhone
	Audio      *audio.Audio
	Recorder   *audio.Recorder
	Polly      polly.Polly
	Definition functions.Definition

	Callstack []string
	HookState bool

	collector        *Collector
	activeDispatcher functions.Dispatcher

	hs           sync.Mutex
	prefixSignal chan bool
}

func New(ph phone.FlowPhone, audio *audio.Audio, rec *audio.Recorder, polly polly.Polly, def functions.Definition) Controller {
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

					// Check if we are in collect mode
					if c.collector == nil {
						// Nope, the normal
						c.handleKey(key)
					} else {
						// Lets collect until we have what we need
						c.handleCollect(key)
					}
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
//
// Yes coral this is the function you scroll over everytime
// putting this here as a visual marker
// to not get lost in this spaghetti codebase
//
//
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
			return
		}
	}

	c.handleAction(action)
}

func (c *Controller) handleCollect(key string) {

	if c.collector.CollectKey(key) || key == "#" {
		err := c.collector.Finish(c)
		c.checkError(err)
		c.collector = nil
	}

}

func (c *Controller) handleAction(action *functions.Action) {
	// Check action type
	actionType, err := action.Type()
	if err != nil {
		c.checkError(err)
		return
	}

	// Get prefix (if set)
	// This err is inverted btw
	// Go users will hate me
	prefix, err := action.GetPrefix()
	if err == nil {
		pr, _ := prefix.GetPlayable()
		pr.Play(c.Audio, c.Polly)
	}

	switch actionType {
	case "fn":
		c.checkError(c.enterFunction(action.Dst))
	case "file":
		c.play(action.File)
	case "randomfile":
		c.play(action.RandomFile)
	case "tts":
		c.play(action.TTS)
	case "srv":
		err := c.runService(action.Service, nil)
		c.checkError(err)
	case "dispatcher":
		q, err := c.Definition.ResolveDispatcher(action.CustomDispatcher)
		c.checkError(err)
		if err == nil {
			c.handleDispatcher(q)
		}
	case "clear":
		c.Audio.Clear()
	}
}

// Appends to the callstack if function exists and tries to schedule prefix
func (c *Controller) enterFunction(dst string) error {
	if newFn, ok := c.Definition.Functions[dst]; ok {
		c.Callstack = append(c.Callstack, newFn.Name)
		c.prefixSignal <- true

		fmt.Println(newFn.Recording)
		if newFn.Recording != (functions.Recording{}) {
			fmt.Println("riga black," + newFn.Recording.Destination)
			c.Recorder.Stop()
			c.Recorder.Record(newFn.Recording.Destination)
		}

		return nil
	}
	return fmt.Errorf("could not find dst function: %v", dst)
}

// Removes from the callstack
func (c *Controller) exitFunction() {
	if len(c.Callstack) > 1 {
		c.Callstack = c.Callstack[:len(c.Callstack)-1]
		c.collector = nil
		c.prefixSignal <- true
	}
}

// Reset Callstack
func (c *Controller) clearCallstack() {
	c.Callstack = c.Callstack[:0]
	c.collector = nil
	c.Callstack = append(c.Callstack, c.Definition.General.Entrypoint)
	c.prefixSignal <- true
}

// Run service
func (c *Controller) runService(srv functions.Service, collector *string) error {
	if _, ok := services.ServiceRegistry[srv.Destination]; !ok {
		return fmt.Errorf("service %s is not loaded", srv.Destination)
	}

	s := services.ServiceRegistry[srv.Destination]

	// Put the controller in collect mode
	inputLength := s.MaxInputLength()
	if inputLength > 0 && collector == nil {
		c.collector = CreateServiceCollector(inputLength, &srv)
		return nil
	}

	input := ""
	if collector != nil {
		input = *collector
	}

	data, err := s.Get(input, srv.Template, srv.Arguments)
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

	// Fix race condition
	if len(c.Callstack) == 0 {
		if c.activeDispatcher != nil {
			c.activeDispatcher.Stop()
		}
		return c.Definition.Functions[c.Definition.General.Entrypoint]
	}

	current := c.Callstack[len(c.Callstack)-1]
	return c.Definition.Functions[current]
}

func (c *Controller) liftHook() {
	c.hs.Lock()
	defer c.hs.Unlock()
	if c.HookState {
		return
	}
	c.HookState = true
	c.Audio.Clear()
	c.collector = nil
	c.enterFunction(c.Definition.General.Entrypoint)

	log.Info("Hook is lifted")
}

func (c *Controller) slamHook() {
	c.hs.Lock()
	defer c.hs.Unlock()
	c.HookState = false
	c.Audio.Clear()

	c.Recorder.Stop()

	c.Callstack = c.Callstack[:0]
	c.collector = nil

	// Check if we're currently in dispatch mode
	// If so, ask dispatcher to exit
	if c.activeDispatcher != nil {
		c.activeDispatcher.Stop()
	}

	log.Info("Hook is slammed")
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

func (c *Controller) handleDispatcher(q functions.Dispatcher) {

	err := q.Load()
	c.checkError(err)

	// Store pointer to queue as dispatcher
	c.activeDispatcher = q

	f := c.activeDispatcher.Start(c.Audio, c.Recorder, c.Polly)

	// Wait for dispatcher to finish
	a := <-f

	// Clear out for GC
	c.activeDispatcher = nil

	_, err = a.Type()
	if err == nil {
		c.handleAction(&a)
	}
}
