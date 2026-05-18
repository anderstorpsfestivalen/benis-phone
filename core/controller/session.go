package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/callctl"
	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
	"github.com/anderstorpsfestivalen/benis-phone/core/phone"
	"github.com/anderstorpsfestivalen/benis-phone/core/tts"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services"
	log "github.com/sirupsen/logrus"
)

// Session represents a single call session with isolated state.
// Each incoming call gets its own Session instance.
type Session struct {
	ID string

	// Per-session components
	Phone    phone.FlowPhone
	Audio    audio.AudioSink
	Recorder audio.AudioSource

	// Shared components (read-only during session)
	TTS        *tts.Registry
	Definition functions.Definition

	// CallControl is the per-call SIP control surface (hangup, transfer,
	// recording, DTMF send). nil in local-audio mode; handlers must check.
	CallControl callctl.Controller

	// Per-session state
	Callstack []string
	HookState bool

	collector        *Collector
	activeDispatcher functions.Dispatcher

	hs           sync.Mutex
	prefixSignal chan bool
	done         chan struct{}
}

// NewSession creates a new call session with the given components.
// callCtl may be nil (e.g. local-audio mode) — call-control handlers will
// surface a friendly error to the caller.
func NewSession(id string, ph phone.FlowPhone, audioSink audio.AudioSink, rec audio.AudioSource, ttsReg *tts.Registry, def functions.Definition, callCtl callctl.Controller) *Session {
	return &Session{
		ID:           id,
		Phone:        ph,
		Audio:        audioSink,
		Recorder:     rec,
		TTS:          ttsReg,
		Definition:   def,
		CallControl:  callCtl,
		prefixSignal: make(chan bool, 100),
		done:         make(chan struct{}),
	}
}

// Start begins the session's main loop, processing hook and key events.
// This method blocks until the session ends (hang up or explicit stop).
func (s *Session) Start() {
	hookchan := s.Phone.GetHookChannel()
	keychan := s.Phone.GetKeyChannel()

	log.WithField("session", s.ID).Info("Session started")

	for {
		select {
		case <-s.done:
			log.WithField("session", s.ID).Info("Session stopped")
			return
		case hook, ok := <-hookchan:
			if !ok {
				// Phone closed its hook channel — call is over. Stop reading
				// it (nil channel disables this case) and wait for Stop() to
				// close s.done. Without this guard a closed channel spins
				// the loop with zero-value false forever.
				hookchan = nil
				continue
			}
			if hook {
				s.liftHook()
			} else {
				s.slamHook()
			}
		case key, ok := <-keychan:
			if !ok {
				keychan = nil
				continue
			}
			log.WithFields(log.Fields{"session": s.ID, "key": key}).Trace("Got key")
			if s.HookState {
				s.Audio.Clear()
				if s.collector == nil {
					s.handleKey(key)
				} else {
					s.handleCollect(key)
				}
			}
		case <-s.prefixSignal:
			err := s.handlePrefix()
			s.checkError(err)
		}
	}
}

// Stop gracefully terminates the session.
func (s *Session) Stop() {
	close(s.done)
	s.Audio.Clear()
	s.Recorder.Stop()
	if s.activeDispatcher != nil {
		s.activeDispatcher.Stop()
	}
}

func (s *Session) handlePrefix() error {
	pr, err := s.getCurrent().Prefix.GetPlayable()
	if err != nil {
		return err
	}
	return pr.Play(s.Audio, s.TTS)
}

func (s *Session) handleKey(key string) {
	if key == "0" {
		if s.getCurrent().ClearCallstack {
			s.clearCallstack()
		} else {
			s.exitFunction()
		}
		return
	}

	action, err := s.getCurrent().ResolveAction(key)
	if err != nil {
		if err.Error() == "could not find key" {
			log.WithFields(log.Fields{"session": s.ID, "key": key}).Trace("Key not found")
			return
		}
		s.checkError(err)
		return
	}

	s.handleAction(action)
}

func (s *Session) handleCollect(key string) {
	if s.collector.CollectKey(key) || key == "#" {
		err := s.collector.Finish(s)
		s.checkError(err)
		s.collector = nil
	}
}

func (s *Session) handleAction(action *functions.Action) {
	actionType, err := action.Type()
	if err != nil {
		s.checkError(err)
		return
	}

	prefix, err := action.GetPrefix()
	if err == nil {
		pr, _ := prefix.GetPlayable()
		pr.Play(s.Audio, s.TTS)
	}

	switch actionType {
	case "fn":
		s.checkError(s.enterFunction(action.Dst))
	case "file":
		s.play(action.File)
	case "randomfile":
		s.play(action.RandomFile)
	case "tts":
		s.play(action.TTS)
	case "srv":
		err := s.runService(action.Service, nil)
		s.checkError(err)
	case "dispatcher":
		q, err := s.Definition.ResolveDispatcher(action.CustomDispatcher)
		s.checkError(err)
		if err == nil {
			s.handleDispatcher(q)
		}
	case "clear":
		s.Audio.Clear()
	case "transfer":
		s.handleTransfer(action.Transfer)
	case "hangup":
		s.handleHangup()
	case "record":
		s.handleRecord(action.Record, action.RecordTo)
	case "dtmf":
		s.handleDTMF(action.DTMF)
	case "livefeed":
		s.handleLiveFeed(action.LiveFeed)
	}
}

func (s *Session) handleTransfer(target string) {
	if s.CallControl == nil {
		s.checkError(fmt.Errorf("transfer not available in this mode"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.CallControl.Transfer(ctx, target); err != nil {
		s.checkError(fmt.Errorf("transfer to %q failed: %w", target, err))
	}
}

func (s *Session) handleHangup() {
	if s.CallControl == nil {
		s.checkError(fmt.Errorf("hangup not available in this mode"))
		return
	}
	s.Audio.Clear()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.CallControl.Hangup(ctx); err != nil {
		log.WithFields(log.Fields{"session": s.ID, "err": err}).Warn("Hangup error (best-effort)")
	}
}

func (s *Session) handleRecord(op, subfolder string) {
	if s.CallControl == nil {
		s.checkError(fmt.Errorf("recording not available in this mode"))
		return
	}
	switch op {
	case "start":
		path, err := s.CallControl.StartRecording(subfolder)
		if err != nil {
			s.checkError(fmt.Errorf("start recording: %w", err))
			return
		}
		log.WithFields(log.Fields{"session": s.ID, "path": path}).Info("Recording started")
	case "stop":
		if err := s.CallControl.StopRecording(); err != nil {
			s.checkError(fmt.Errorf("stop recording: %w", err))
		}
	default:
		s.checkError(fmt.Errorf("unknown record op %q (want start|stop)", op))
	}
}

func (s *Session) handleDTMF(digits string) {
	if s.CallControl == nil {
		s.checkError(fmt.Errorf("dtmf send not available in this mode"))
		return
	}
	if err := s.CallControl.SendDTMF(digits); err != nil {
		s.checkError(fmt.Errorf("send dtmf %q: %w", digits, err))
	}
}

func (s *Session) handleLiveFeed(cfg *functions.LiveFeed) {
	if cfg == nil {
		s.checkError(fmt.Errorf("livefeed: missing config"))
		return
	}
	src, err := audio.NewCaptureSource(cfg.Device, cfg.Channel)
	if err != nil {
		s.checkError(fmt.Errorf("livefeed: %w", err))
		return
	}
	log.WithFields(log.Fields{
		"session": s.ID,
		"device":  cfg.Device,
		"channel": cfg.Channel,
	}).Info("Livefeed started")
	if err := s.Audio.PlaySource(src); err != nil {
		_ = src.Close()
		s.checkError(fmt.Errorf("livefeed submit: %w", err))
	}
}

func (s *Session) enterFunction(dst string) error {
	if newFn, ok := s.Definition.Functions[dst]; ok {
		s.Callstack = append(s.Callstack, newFn.Name)
		s.prefixSignal <- true

		if newFn.Recording != (functions.Recording{}) {
			s.Recorder.Stop()
			s.Recorder.Record(newFn.Recording.Destination)
		}

		return nil
	}
	return fmt.Errorf("could not find dst function: %v", dst)
}

func (s *Session) exitFunction() {
	if len(s.Callstack) > 1 {
		s.Callstack = s.Callstack[:len(s.Callstack)-1]
		s.collector = nil
		s.prefixSignal <- true
	}
}

func (s *Session) clearCallstack() {
	s.Callstack = s.Callstack[:0]
	s.collector = nil
	s.Callstack = append(s.Callstack, s.Definition.General.Entrypoint)
	s.prefixSignal <- true
}

func (s *Session) runService(srv functions.Service, collector *string) error {
	if _, ok := services.ServiceRegistry[srv.Destination]; !ok {
		return fmt.Errorf("service %s is not loaded", srv.Destination)
	}

	svc := services.ServiceRegistry[srv.Destination]

	inputLength := svc.MaxInputLength()
	if inputLength > 0 && collector == nil {
		s.collector = CreateServiceCollector(inputLength, &srv)
		return nil
	}

	input := ""
	if collector != nil {
		input = *collector
	}

	data, err := svc.Get(input, srv.Template, srv.Arguments)
	if err != nil {
		return err
	}

	t := s.Definition.StandardTTS(data)
	if srv.TTS != (functions.TTS{}) {
		t = srv.TTS
		t.Message = data
	}

	s.play(t)

	return nil
}

func (s *Session) getCurrent() *functions.Fn {
	if len(s.Callstack) == 0 {
		if s.activeDispatcher != nil {
			s.activeDispatcher.Stop()
		}
		return s.Definition.Functions[s.Definition.General.Entrypoint]
	}

	current := s.Callstack[len(s.Callstack)-1]
	return s.Definition.Functions[current]
}

func (s *Session) liftHook() {
	s.hs.Lock()
	defer s.hs.Unlock()
	if s.HookState {
		return
	}
	s.HookState = true
	s.Audio.Clear()
	s.collector = nil
	s.enterFunction(s.Definition.General.Entrypoint)

	log.WithField("session", s.ID).Info("Hook lifted")
}

func (s *Session) slamHook() {
	s.hs.Lock()
	defer s.hs.Unlock()
	s.HookState = false
	s.Audio.Clear()

	s.Recorder.Stop()

	s.Callstack = s.Callstack[:0]
	s.collector = nil

	if s.activeDispatcher != nil {
		s.activeDispatcher.Stop()
	}

	log.WithField("session", s.ID).Info("Hook slammed")
}

func (s *Session) play(pl functions.PlayGenerator) {
	p := functions.CreatePlayable(pl)
	log.WithField("session", s.ID).Trace("Playing")
	p.Play(s.Audio, s.TTS)
}

func (s *Session) checkError(e error) {
	if e != nil {
		log.WithField("session", s.ID).Error(e)
		s.play(s.Definition.EnglishTTS(e.Error()))
	}
}

func (s *Session) handleDispatcher(q functions.Dispatcher) {
	err := q.Load()
	s.checkError(err)

	s.activeDispatcher = q

	f := s.activeDispatcher.Start(s.Audio, s.Recorder, s.TTS)

	a := <-f

	s.activeDispatcher = nil

	_, err = a.Type()
	if err == nil {
		s.handleAction(&a)
	}
}
