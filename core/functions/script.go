package functions

import (
	"strings"

	"github.com/dop251/goja"
)

// Script is an action that runs an inline JavaScript program (via goja) to
// drive a complete, stateful IVR dialogue. Where GenericJSON fetches data and
// speaks exactly one templated string, a Script expresses arbitrary flow logic
// as ordinary imperative code — fetch APIs, speak, wait for keypresses, branch,
// loop — which is far easier to read than a graph of listmenu/genericjson nodes
// chained through .Vars.
//
// The controller runs the compiled program in its own goroutine with a small,
// synchronous API bound into the runtime (see core/controller/script.go):
//
//	speak(text)                     – synth TTS + play, blocks until done
//	readKey()                       – block for a DTMF key; "0".."9"/"*"/"#" or null on timeout
//	http.get(url, opts?)            – returns { status, json, text }; json is a native JS value
//	http.post(url, bodyObjOrStr, opts?)
//	vars.get(name) / vars.set(name, value)  – the shared .Vars blackboard
//	args                            – the TOML `args` map (read-only)
//	goto(fnName, param?)            – on exit, enter fnName; param is stored in .Vars.goto
//	log(...)                        – trace logging
//
// Because IVR flows are inherently synchronous, goja's blocking single-threaded
// model fits perfectly — no promises or event loop. HTTP responses come back as
// native parsed JS objects, so scripts navigate data with plain JavaScript
// (.find/.filter/?.) rather than jq. goja targets ES5.1 plus much of ES6.
//
// TOML usage (inside an action) — the code is normally a '''triple-quoted''' literal:
//
//	[[fn.actions]]
//	num = 4
//	name = "Öl"
//	[fn.actions.script]
//	args = { base_url = "https://beer.anderstorpsfestivalen.se" }
//	code = '''
//	  var members = http.get(args.base_url + "/api/members").json;
//	  ...
//	'''
type Script struct {
	// Code is the inline JavaScript program.
	Code string `toml:"code"`

	// Args are static config values exposed to the program as the global
	// `args` object (all string-valued). Mirrors Interactive.Arguments.
	Args map[string]string `toml:"args"`

	// TTS overrides the voice/lang/engine/provider used by speak(), falling
	// back to the definition defaults via ResolveTTS at call time.
	TTS TTS `toml:"tts"`

	// program is the compiled form, cached so a per-call runtime just runs a
	// pre-parsed *goja.Program instead of re-parsing the source every call.
	// Populated by Compile() from Definition.Prepare (single-threaded). A
	// *goja.Program is immutable and safe to run from many per-call runtimes
	// concurrently, so this plain pointer needs no lock — it must only be set
	// before any call runs and never mutated afterward. Not serialized; it's
	// unexported so BurntSushi/toml and the typegen AST walker both skip it.
	program *goja.Program
	// compErr records a compile failure so the controller can surface the
	// message when it finds no program at call time. Set once in Compile
	// (single-threaded, from Prepare); read-only afterward.
	compErr error
}

// scriptWrapPrefix / scriptWrapSuffix wrap the author's code in an immediately
// invoked function so a top-level `return` (natural for an early exit) is legal
// — goja's RunProgram runs code as a *script*, where a top-level return is a
// SyntaxError, whereas the editor's browser test harness wraps the code in a
// `new Function(...)` (a function body). Wrapping here keeps the two engines'
// semantics identical. The leading newline keeps a first-line `//` comment from
// swallowing the code; it shifts goja's reported error lines by one, which the
// editor's own (unwrapped) CodeMirror linter doesn't suffer from.
const scriptWrapPrefix = "(function(){\n"
const scriptWrapSuffix = "\n})()"

// Compile parses and caches the program. Called from Definition.Prepare so
// syntax errors surface at config-load time. Blank code is a no-op (program
// stays nil) and returns no error.
func (s *Script) Compile() error {
	if strings.TrimSpace(s.Code) == "" {
		return nil
	}
	p, err := goja.Compile("script", scriptWrapPrefix+s.Code+scriptWrapSuffix, false)
	if err != nil {
		s.compErr = err
		return err
	}
	s.program = p
	return nil
}

// Program returns the cached compiled program (nil if the script is blank or
// failed to compile).
func (s *Script) Program() *goja.Program { return s.program }

// CompileErr returns the compile error recorded by Compile, if any.
func (s *Script) CompileErr() error { return s.compErr }
