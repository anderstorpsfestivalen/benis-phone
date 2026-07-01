package functions

// Interactive selects a stateful, multi-step call flow implemented in Go and
// registered by name (see extensions/interactive). Unlike Service/GenericJSON —
// which fetch data and speak exactly one string — an interactive handler drives
// the caller through a dynamic dialogue: it can build a menu from a live API
// response, collect follow-up keypresses, thread captured state between HTTP
// calls, and branch. The controller hands it a per-call IO surface (Speak +
// NextKey) and runs it to completion in its own goroutine.
//
// TOML usage (inside an action):
//
//	interactive = { dst = "beer", args = { base_url = "https://beer.anderstorpsfestivalen.se" } }
type Interactive struct {
	// Destination is the registry name of the handler to run, e.g. "beer".
	Destination string `toml:"dst"`
	// Arguments are handler-specific config passed straight through, e.g.
	// base_url. Mirrors Service.Arguments.
	Arguments map[string]string `toml:"args"`
	// TTS overrides the voice/lang/engine/provider used for everything the
	// handler speaks, falling back to the definition defaults.
	TTS TTS `toml:"tts"`
}
