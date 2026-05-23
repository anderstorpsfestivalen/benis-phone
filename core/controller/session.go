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

	// callCtx scopes long-running per-call work (HTTP fetches, TTS synth)
	// to the current hook cycle. liftHook installs a fresh ctx; slamHook
	// and Stop cancel it so in-flight goroutines stop wasting cycles —
	// crucial when a caller hangs up mid-fetch and we'd otherwise still
	// burn TTS budget synthesizing audio for a dead call.
	callCtx    context.Context
	callCancel context.CancelFunc
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
	if s.callCancel != nil {
		s.callCancel()
	}
	if s.activeDispatcher != nil {
		s.activeDispatcher.Stop()
	}
}

// fetchCtx returns the per-call context used to scope HTTP fetches and
// background TTS synthesis. Falls back to a fresh background context if
// no call is in progress (defensive — callers always run after liftHook).
func (s *Session) fetchCtx() context.Context {
	if s.callCtx != nil {
		return s.callCtx
	}
	return context.Background()
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
		err := s.runServiceWithPmsg(action.Service, nil, action.Pmsg)
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
	case "genericjson":
		err := s.runGenericJSONWithPmsg(action.GenericJSON, action.Pmsg)
		s.checkError(err)
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

// runService dispatches a service call with no pmsg. Used by the collector
// path (digits-then-service) where there's no parallel pmsg to play.
func (s *Session) runService(srv functions.Service, collector *string) error {
	return s.runServiceWithPmsg(srv, collector, functions.Prefix{})
}

// runServiceWithPmsg dispatches a service call, optionally playing a pmsg in
// parallel with the slow work (svc.Get + TTS synthesis). Sequence:
//
//  1. If the service still needs digits, install a collector and return.
//  2. Start a goroutine that fetches data and pre-synthesizes the result TTS
//     into mp3 bytes.
//  3. Synchronously play the pmsg (Wait=true), so the OutputStream queues it
//     ahead of whatever we submit next and we know exactly when it ends.
//  4. Once the goroutine finishes, fire-and-forget the result audio.
//
// With no pmsg this just blocks on prepare and submits the result async,
// matching the original behavior.
func (s *Session) runServiceWithPmsg(srv functions.Service, collector *string, pmsg functions.Prefix) error {
	svc, ok := services.ServiceRegistry[srv.Destination]
	if !ok {
		return fmt.Errorf("service %s is not loaded", srv.Destination)
	}

	inputLength := svc.MaxInputLength()
	if inputLength > 0 && collector == nil {
		s.collector = CreateServiceCollector(inputLength, &srv)
		return nil
	}

	input := ""
	if collector != nil {
		input = *collector
	}

	ctx := s.fetchCtx()
	type prepResult struct {
		audio []byte
		err   error
	}
	resultCh := make(chan prepResult, 1)
	go func() {
		audio, err := s.prepareServiceAudio(ctx, svc, srv, input)
		resultCh <- prepResult{audio, err}
	}()

	// Play pmsg (if any) synchronously. Clear=false: don't cut off whatever
	// preceded us (e.g. the action's regular Prefix still playing); Wait=true:
	// block until pmsg playback finishes so the result is guaranteed to land
	// in the output queue after it.
	if pmsg != (functions.Prefix{}) {
		pmsgPl, perr := pmsg.GetPlayable()
		if perr == nil {
			pmsgPl.Wait = true
			pmsgPl.Clear = false
			if err := pmsgPl.Play(s.Audio, s.TTS); err != nil {
				log.WithField("session", s.ID).Warnf("pmsg play: %v", err)
			}
		} else {
			log.WithField("session", s.ID).Warnf("pmsg: %v", perr)
		}
	}

	select {
	case <-ctx.Done():
		// Caller hung up during the service call — drain the result so
		// the goroutine can exit, but don't play anything.
		go func() { <-resultCh }()
		return nil
	case r := <-resultCh:
		if r.err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return r.err
		}
		go func() {
			if err := s.Audio.PlayFromStream(r.audio); err != nil {
				log.WithField("session", s.ID).Warnf("service result play: %v", err)
			}
		}()
		return nil
	}
}

// runGenericJSONWithPmsg fetches a configurable JSON endpoint, renders its
// text/template, and speaks the result. Mirrors runServiceWithPmsg: the
// fetch + TTS synthesis run in a goroutine while the optional Pmsg plays
// in the foreground, so callers aren't stuck in silence while the request
// is in flight. The goroutine is scoped to the current call's context so
// a mid-fetch hangup cancels both the HTTP request and any subsequent TTS
// work, instead of wasting Polly/ElevenLabs budget on a dead call.
func (s *Session) runGenericJSONWithPmsg(g functions.GenericJSON, pmsg functions.Prefix) error {
	ctx := s.fetchCtx()
	type prepResult struct {
		audio []byte
		err   error
	}
	resultCh := make(chan prepResult, 1)
	go func() {
		audio, err := s.prepareGenericJSONAudio(ctx, g)
		resultCh <- prepResult{audio, err}
	}()

	if pmsg != (functions.Prefix{}) {
		pmsgPl, perr := pmsg.GetPlayable()
		if perr == nil {
			pmsgPl.Wait = true
			pmsgPl.Clear = false
			if err := pmsgPl.Play(s.Audio, s.TTS); err != nil {
				log.WithField("session", s.ID).Warnf("pmsg play: %v", err)
			}
		} else {
			log.WithField("session", s.ID).Warnf("pmsg: %v", perr)
		}
	}

	select {
	case <-ctx.Done():
		// Call ended while we waited for the fetch — drop the result on
		// the floor when it eventually arrives; the consuming goroutine
		// will read from resultCh once and exit.
		go func() { <-resultCh }()
		return nil
	case r := <-resultCh:
		if r.err != nil {
			if ctx.Err() != nil {
				return nil // hangup during fetch; don't speak an error
			}
			return r.err
		}
		go func() {
			if err := s.Audio.PlayFromStream(r.audio); err != nil {
				log.WithField("session", s.ID).Warnf("genericjson result play: %v", err)
			}
		}()
		return nil
	}
}

// prepareGenericJSONAudio fetches+renders the JSON node and synthesizes
// the output through TTS, returning mp3 bytes ready to feed the audio
// sink. Voice/lang/engine/provider come from the node's TTS overrides
// when set, otherwise fall back to the definition's StandardTTS.
func (s *Session) prepareGenericJSONAudio(ctx context.Context, g functions.GenericJSON) ([]byte, error) {
	rendered, err := g.FetchAndRender(ctx)
	if err != nil {
		return nil, err
	}

	t := s.Definition.ResolveTTS(g.TTS, rendered)
	return s.TTS.Synthesize(t.Provider, tts.Request{
		Message:  t.Message,
		Voice:    t.Voice,
		Language: t.Language,
		Engine:   t.Engine,
	})
}

// prepareServiceAudio runs the service and synthesizes its result into mp3
// bytes. Safe to call from a goroutine: it touches the service, the TTS
// registry, and the session's read-only Definition, but never the audio sink.
// ctx scopes the work to the current call — if the caller hangs up we bail
// before paying for the TTS synthesis (and individual services that thread
// the context through their HTTP clients can cancel mid-fetch too).
func (s *Session) prepareServiceAudio(ctx context.Context, svc services.Service, srv functions.Service, input string) ([]byte, error) {
	data, err := svc.Get(input, srv.Template, srv.Arguments)
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	t := s.Definition.ResolveTTS(srv.TTS, data)
	return s.TTS.Synthesize(t.Provider, tts.Request{
		Message:  t.Message,
		Voice:    t.Voice,
		Language: t.Language,
		Engine:   t.Engine,
	})
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
	s.callCtx, s.callCancel = context.WithCancel(context.Background())
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

	if s.callCancel != nil {
		s.callCancel()
		s.callCancel = nil
		s.callCtx = nil
	}

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
