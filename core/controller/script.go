package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
	"github.com/dop251/goja"
	log "github.com/sirupsen/logrus"
)

// handleScript runs a script action's inline JavaScript program in its own
// goroutine so the session loop keeps draining hook/key events — which is how
// readKey() and barge-in work. When the program returns, the goroutine signals
// scriptDone with the goto() target (empty = return to the calling menu) and
// the loop hands control back accordingly.
func (s *Session) handleScript(action *functions.Action) {
	prog := action.Script.Program()
	if prog == nil {
		if err := action.Script.CompileErr(); err != nil {
			s.checkError(fmt.Errorf("script %q: %w", action.Name, err))
		} else {
			s.checkError(fmt.Errorf("script %q has no compiled program", action.Name))
		}
		return
	}

	// Drop keys buffered from before the script started (e.g. the digit that
	// selected this action) so the first readKey() blocks cleanly.
drain:
	for {
		select {
		case <-s.scriptKeys:
		default:
			break drain
		}
	}

	s.activeScript = true
	io := &scriptIO{session: s, tts: action.Script.TTS}
	ctx := s.fetchCtx()
	args := action.Script.Args

	go func() {
		dst := s.runScript(ctx, io, prog, args)
		// Signal completion. Guard on s.done so a torn-down session doesn't
		// block this goroutine forever if the loop has already exited.
		select {
		case s.scriptDone <- dst:
		case <-s.done:
		}
	}()
}

// scriptIO is the per-call surface a script uses to talk to the caller: synth
// TTS and play (Speak), and block for a DTMF key (NextKey). It only touches the
// audio sink, the TTS registry, the read-only Definition, and the scriptKeys
// channel — all safe to use from the script goroutine.
type scriptIO struct {
	session *Session
	tts     functions.TTS
}

// Speak synthesizes text (honoring the script's TTS overrides, then definition
// defaults) and plays it to the caller, blocking until playback finishes or a
// barge-in Clear() cuts it short.
func (io *scriptIO) Speak(ctx context.Context, text string) error {
	data, err := io.session.synth(io.tts, text)
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return io.session.Audio.PlayFromStream(data)
}

// NextKey blocks for the next DTMF key, giving up on hangup (ctx) or after
// scriptKeyTimeout of silence.
func (io *scriptIO) NextKey(ctx context.Context) (string, error) {
	select {
	case k := <-io.session.scriptKeys:
		return k, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(scriptKeyTimeout):
		return "", fmt.Errorf("timeout waiting for key")
	}
}

// runScript builds a fresh per-call goja runtime (runtimes aren't
// goroutine-safe; each call already has its own goroutine), binds the small
// synchronous API a flow needs, runs the pre-compiled program, and returns the
// fn name the script asked to goto() (empty if none). All bound functions block
// the single JS goroutine, which is exactly what we want for a synchronous IVR
// dialogue.
func (s *Session) runScript(ctx context.Context, io *scriptIO, prog *goja.Program, args map[string]string) (gotoTarget string) {
	vm := goja.New()

	// Interrupt the VM if the caller hangs up, so a pure-JS infinite loop (or a
	// script that swallows a blocking call's error and spins) can't pin this
	// goroutine open. Interrupts are checked between VM operations.
	stopWatch := make(chan struct{})
	defer close(stopWatch)
	go func() {
		select {
		case <-ctx.Done():
			vm.Interrupt("hangup")
		case <-stopWatch:
		}
	}()

	// speak(text): synth + play, blocking until done. A returned error becomes
	// a thrown JS exception (goja auto-converts a trailing error return).
	_ = vm.Set("speak", func(text string) error {
		return io.Speak(ctx, text)
	})

	// readKey(): block for a DTMF key; returns "0".."9"/"*"/"#", or null on
	// timeout. A hangup interrupts the VM via the watchdog above.
	_ = vm.Set("readKey", func() goja.Value {
		k, err := io.NextKey(ctx)
		if err != nil {
			return goja.Null()
		}
		return vm.ToValue(k)
	})

	// http.get(url, opts?) / http.post(url, bodyObjOrString, opts?):
	// returns { status, json, text }. json is a native JS value (null if the
	// body isn't JSON). A transport error is thrown as a JS exception.
	doHTTP := func(method string, call goja.FunctionCall) goja.Value {
		url := call.Argument(0).String()
		var body string
		var opts goja.Value
		if method == "POST" {
			body = scriptBody(call.Argument(1))
			opts = call.Argument(2)
		} else {
			opts = call.Argument(1)
		}
		headers, timeout := scriptHTTPOpts(opts)
		data, status, raw, err := functions.DoScriptRequest(ctx, method, url, body, headers, timeout)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		res := vm.NewObject()
		_ = res.Set("status", status)
		_ = res.Set("json", vm.ToValue(data))
		_ = res.Set("text", raw)
		return res
	}
	httpObj := vm.NewObject()
	_ = httpObj.Set("get", func(call goja.FunctionCall) goja.Value { return doHTTP("GET", call) })
	_ = httpObj.Set("post", func(call goja.FunctionCall) goja.Value { return doHTTP("POST", call) })
	_ = vm.Set("http", httpObj)

	// vars.get(name) / vars.set(name, value): the shared .Vars blackboard, so a
	// script and a genericjson node interoperate on the same flow state.
	varsObj := vm.NewObject()
	_ = varsObj.Set("get", func(name string) goja.Value {
		return vm.ToValue(s.VarsSnapshot()[name])
	})
	_ = varsObj.Set("set", func(name string, value goja.Value) {
		s.SetVars(map[string]any{name: value.Export()})
	})
	_ = vm.Set("vars", varsObj)

	// args: the TOML `args` map (string-valued), read-only from the script's POV.
	_ = vm.Set("args", args)

	// goto(fnName, param?): record the exit destination; if param is given,
	// store it under .Vars.goto so the target fn's genericjson templates (or a
	// downstream script) can read it — matching how listmenu used to pass the
	// selected item forward.
	_ = vm.Set("goto", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) > 0 {
			gotoTarget = call.Argument(0).String()
		}
		if len(call.Arguments) > 1 {
			if p := call.Argument(1); !goja.IsUndefined(p) && !goja.IsNull(p) {
				s.SetVars(map[string]any{"goto": p.Export()})
			}
		}
		return goja.Undefined()
	})

	// log(...): trace logging for debugging a flow.
	_ = vm.Set("log", func(call goja.FunctionCall) goja.Value {
		parts := make([]interface{}, 0, len(call.Arguments))
		for _, a := range call.Arguments {
			parts = append(parts, a.String())
		}
		log.WithField("session", s.ID).Trace(parts...)
		return goja.Undefined()
	})

	if _, err := vm.RunProgram(prog); err != nil {
		// A hangup unwinds via Interrupt/ctx — not a real error to report.
		var ie *goja.InterruptedError
		if ctx.Err() != nil || errors.As(err, &ie) {
			return gotoTarget
		}
		log.WithField("session", s.ID).Warnf("script error: %v", err)
	}
	return gotoTarget
}

// scriptBody coerces an http.post body argument into a request body string:
// a JS string is sent verbatim; anything else is JSON-encoded.
func scriptBody(v goja.Value) string {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return ""
	}
	exported := v.Export()
	if str, ok := exported.(string); ok {
		return str
	}
	b, err := json.Marshal(exported)
	if err != nil {
		return ""
	}
	return string(b)
}

// scriptHTTPOpts reads the optional opts object of an http.* call:
// { headers: {..}, timeout: <seconds> }.
func scriptHTTPOpts(v goja.Value) (map[string]string, time.Duration) {
	headers := map[string]string{}
	var timeout time.Duration
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return headers, timeout
	}
	m, ok := v.Export().(map[string]interface{})
	if !ok {
		return headers, timeout
	}
	if h, ok := m["headers"].(map[string]interface{}); ok {
		for k, hv := range h {
			headers[k] = fmt.Sprintf("%v", hv)
		}
	}
	if t, ok := m["timeout"].(float64); ok && t > 0 {
		timeout = time.Duration(t) * time.Second
	}
	return headers, timeout
}
