package controller

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
	"github.com/anderstorpsfestivalen/benis-phone/core/tts"
	"github.com/faiface/beep"
)

// fakeSink records everything played and captures the mp3 bytes so a test can
// assert what the script spoke (the TTS provider below turns the text into
// bytes we can decode back).
type fakeSink struct {
	mu     sync.Mutex
	played [][]byte
}

func (f *fakeSink) PlayFromFile(string) error { return nil }
func (f *fakeSink) PlayFromStream(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.played = append(f.played, data)
	return nil
}
func (f *fakeSink) Clear()          {}
func (f *fakeSink) IsPlaying() bool { return false }
func (f *fakeSink) ExternalPlayback(beep.StreamSeekCloser, beep.Format) {}
func (f *fakeSink) PlaySource(audio.Source) error                       { return nil }

func (f *fakeSink) spoken() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.played))
	for _, b := range f.played {
		out = append(out, string(b))
	}
	return out
}

// echoProvider "synthesizes" by returning the message text as bytes, so the
// sink's captured bytes are exactly what speak() was asked to say.
type echoProvider struct{}

func (echoProvider) Name() string { return "echo" }
func (echoProvider) CacheKey(req tts.Request) string {
	// Per-message key so two different speaks don't collide in the on-disk
	// cache (which would make the second speak echo the first's bytes).
	return fmt.Sprintf("%x", sha1.Sum([]byte(req.Message)))
}
func (echoProvider) Synthesize(req tts.Request) ([]byte, error) { return []byte(req.Message), nil }

func newScriptSession(t *testing.T) (*Session, *fakeSink) {
	t.Helper()
	reg := tts.NewRegistry(t.TempDir(), "echo")
	reg.Register(echoProvider{})
	sink := &fakeSink{}
	return &Session{
		ID:         "test",
		Audio:      sink,
		TTS:        reg,
		scriptKeys: make(chan string, 8),
	}, sink
}

// runScriptForTest compiles code and runs it through the goja bridge with a
// short-lived context, returning the goto target.
func runScriptForTest(t *testing.T, s *Session, code string, args map[string]string) string {
	t.Helper()
	sc := functions.Script{Code: code, Args: args}
	if err := sc.Compile(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	io := &scriptIO{session: s}
	return s.runScript(ctx, io, sc.Program(), args)
}

func TestScriptHTTPVarsAndGoto(t *testing.T) {
	// A tiny API: GET /members returns two members; POST /roll echoes the body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/members":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 1, "name": "Anna"}, {"id": 2, "name": "Bo"},
			})
		case "/roll":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			_ = json.NewEncoder(w).Encode(map[string]any{"userId": body["userId"], "beer": "IPA"})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	s, sink := newScriptSession(t)

	code := `
		var members = http.get(args.base + "/members").json;
		if (members.length !== 2) { throw new Error("expected 2 members"); }
		var chosen = members.find(function (m) { return m.name === "Bo"; });
		speak("Vald: " + chosen.name);
		vars.set("member", chosen);
		var roll = http.post(args.base + "/roll", { userId: chosen.id }).json;
		speak("Rullade " + roll.beer + " for " + roll.userId);
		goto("beer_decide", { rollUser: roll.userId });
	`
	dst := runScriptForTest(t, s, code, map[string]string{"base": srv.URL})

	if dst != "beer_decide" {
		t.Fatalf("goto target = %q, want beer_decide", dst)
	}
	spoken := sink.spoken()
	want := []string{"Vald: Bo", "Rullade IPA for 2"}
	if len(spoken) != len(want) {
		t.Fatalf("spoken = %v, want %v", spoken, want)
	}
	for i := range want {
		if spoken[i] != want[i] {
			t.Fatalf("spoken[%d] = %q, want %q", i, spoken[i], want[i])
		}
	}
	// goto's param is stored under .Vars.goto; the chosen member under "member".
	if g, _ := s.VarsSnapshot()["goto"].(map[string]interface{}); g == nil || g["rollUser"] == nil {
		t.Fatalf("expected .Vars.goto.rollUser to be set, got %#v", s.VarsSnapshot()["goto"])
	}
	if m, _ := s.VarsSnapshot()["member"].(map[string]interface{}); m == nil || m["name"] != "Bo" {
		t.Fatalf("expected .Vars.member.name == Bo, got %#v", s.VarsSnapshot()["member"])
	}
}

func TestScriptReadKey(t *testing.T) {
	s, sink := newScriptSession(t)
	// Pre-load a keypress so readKey() returns immediately.
	s.scriptKeys <- "2"

	code := `
		var k = readKey();
		if (k === "2") { speak("two"); } else { speak("other"); }
	`
	runScriptForTest(t, s, code, nil)

	spoken := sink.spoken()
	if len(spoken) != 1 || spoken[0] != "two" {
		t.Fatalf("spoken = %v, want [two]", spoken)
	}
}

func TestScriptTopLevelReturn(t *testing.T) {
	// A top-level `return` is a natural early-exit; the IIFE wrap in
	// Script.Compile makes it legal in goja (which runs code as a script).
	s, sink := newScriptSession(t)
	code := `
		speak("before");
		if (true) { return; }
		speak("after");
	`
	runScriptForTest(t, s, code, nil)
	spoken := sink.spoken()
	if len(spoken) != 1 || spoken[0] != "before" {
		t.Fatalf("spoken = %v, want [before] (return should stop the script)", spoken)
	}
}

func TestScriptCompileError(t *testing.T) {
	sc := functions.Script{Code: "this is ( not valid javascript"}
	if err := sc.Compile(); err == nil {
		t.Fatal("expected compile error for invalid JS")
	}
	if sc.Program() != nil {
		t.Fatal("expected nil program on compile error")
	}
}
