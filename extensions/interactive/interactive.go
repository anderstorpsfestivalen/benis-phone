// Package interactive defines the contract for stateful, multi-step IVR call
// flows and the registry that maps a name (from the TOML `interactive.dst`
// field) to a handler.
//
// Where Service / GenericJSON fetch data and speak exactly one string, an
// interactive Handler drives a live dialogue: it can build a menu from an API
// response, collect follow-up keypresses, thread captured state between HTTP
// calls, and branch. The controller runs the handler in its own goroutine and
// hands it an IO surface bound to the call.
//
// Handlers register themselves from an init() via Register, and the package
// that owns them is blank-imported by the controller (mirroring how the
// controller already depends on extensions/services). Keeping the interfaces
// in this leaf package — and registering by init rather than a static literal
// referencing the concrete handlers — avoids an import cycle between the
// registry and the handler packages (which must import IO here).
package interactive

import "context"

// IO is the per-call surface a Handler uses to talk to the caller. It is
// implemented by the controller and bound to a single Session. All methods
// block and honor ctx cancellation (caller hangup), returning ctx.Err() so the
// handler can unwind promptly.
type IO interface {
	// Speak synthesizes text through TTS and plays it to the caller,
	// blocking until playback finishes (or ctx is cancelled).
	Speak(ctx context.Context, text string) error
	// NextKey blocks until the caller presses a DTMF key and returns it
	// ("0".."9", "*", "#"). Returns a non-nil error on hangup (ctx done) or
	// if the caller stays silent past an internal timeout.
	NextKey(ctx context.Context) (string, error)
}

// Handler is a named interactive flow. Run drives the whole dialogue and
// returns when the flow is done (control returns to the menu the caller came
// from) or ctx is cancelled. args are the handler-specific TOML `args`.
type Handler interface {
	Run(ctx context.Context, io IO, args map[string]string) error
}

// Registry maps a flow name to its Handler. Populated by handler packages'
// init() via Register.
var Registry = map[string]Handler{}

// Register adds a handler under name. Intended to be called from a handler
// package's init(). A later registration with the same name wins.
func Register(name string, h Handler) {
	Registry[name] = h
}

// Lookup returns the handler registered under name.
func Lookup(name string) (Handler, bool) {
	h, ok := Registry[name]
	return h, ok
}
