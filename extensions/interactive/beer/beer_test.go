package beer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// stubIO scripts a caller: Speak records everything spoken; NextKey returns the
// next queued key. When keys run out NextKey returns io.EOF, which Run treats
// as a benign exit.
type stubIO struct {
	mu     sync.Mutex
	spoken []string
	keys   []string
	i      int
}

func (s *stubIO) Speak(_ context.Context, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spoken = append(s.spoken, text)
	return nil
}

func (s *stubIO) NextKey(_ context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.i >= len(s.keys) {
		return "", io.EOF
	}
	k := s.keys[s.i]
	s.i++
	return k, nil
}

func (s *stubIO) said() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.spoken, " || ")
}

// recorded captures the requests the server saw, so tests can assert the flow
// hit the right endpoints with the right bodies.
type recorded struct {
	method string
	path   string
	body   string
}

func newServer(t *testing.T, rolls *[]recorded, mu *sync.Mutex) *httptest.Server {
	t.Helper()
	members := `[{"id":13,"name":"alexfoo"},{"id":11,"name":"coral"},{"id":14,"name":"fozzie"},{"id":15,"name":"summa"},{"id":12,"name":"wberg"}]`
	bac := `{"members":[{"userId":13,"username":"alexfoo","promille":0},{"userId":11,"username":"coral","promille":0.054},{"userId":12,"username":"wberg","promille":0.057}]}`
	roll := `{"id":103,"userId":12,"username":"wberg","productNameBold":"Pelles","productNameThin":"Pilsner","producerName":"Åbro Bryggeri","alcoholPercent":5}`

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		*rolls = append(*rolls, recorded{method: r.Method, path: r.URL.Path, body: strings.TrimSpace(string(b))})
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/members":
			io.WriteString(w, members)
		case r.URL.Path == "/api/public/bac":
			io.WriteString(w, bac)
		case r.URL.Path == "/api/public/roll":
			io.WriteString(w, roll)
		case strings.HasPrefix(r.URL.Path, "/api/public/roll/"):
			io.WriteString(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestBeerMenuAndBACMapping(t *testing.T) {
	var reqs []recorded
	var mu sync.Mutex
	srv := newServer(t, &reqs, &mu)
	defer srv.Close()

	// Press 2 → sorted-by-id index 1 = wberg (id 12). Then 0 to back out of
	// the roll offer so we only exercise the menu + BAC readout.
	sio := &stubIO{keys: []string{"2", "0"}}
	if err := (&Beer{}).Run(context.Background(), sio, map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	said := sio.said()
	// Menu is built id-ascending: coral(11), wberg(12), alexfoo(13), fozzie(14), summa(15).
	if !strings.Contains(said, "Tryck 1 för coral") || !strings.Contains(said, "Tryck 2 för wberg") {
		t.Errorf("menu wrong: %q", said)
	}
	// wberg's promille is 0.057 → "0,057".
	if !strings.Contains(said, "wberg, din alkoholhalt är 0,057 promille") {
		t.Errorf("BAC readout wrong: %q", said)
	}
	mu.Lock()
	defer mu.Unlock()
	if reqs[0].path != "/api/members" || reqs[1].path != "/api/public/bac" {
		t.Errorf("unexpected request order: %+v", reqs)
	}
}

func TestBeerRollAccept(t *testing.T) {
	var reqs []recorded
	var mu sync.Mutex
	srv := newServer(t, &reqs, &mu)
	defer srv.Close()

	// Press 2 (wberg, id 12) → 1 (roll) → 1 (accept).
	sio := &stubIO{keys: []string{"2", "1", "1"}}
	if err := (&Beer{}).Run(context.Background(), sio, map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	said := sio.said()
	if !strings.Contains(said, "Du rullade Pelles, 5 procent") {
		t.Errorf("roll readout wrong: %q", said)
	}
	if !strings.Contains(said, "Accepterad") {
		t.Errorf("expected accept confirmation: %q", said)
	}

	mu.Lock()
	defer mu.Unlock()
	// Verify the roll POST carried the chosen userId and accept hit the turn id.
	var sawRoll, sawAccept bool
	for _, r := range reqs {
		if r.method == http.MethodPost && r.path == "/api/public/roll" {
			sawRoll = true
			var body map[string]int
			if err := json.Unmarshal([]byte(r.body), &body); err != nil || body["userId"] != 12 {
				t.Errorf("roll body wrong: %q (err %v)", r.body, err)
			}
		}
		if r.method == http.MethodPost && r.path == "/api/public/roll/103/accept" {
			sawAccept = true
		}
	}
	if !sawRoll || !sawAccept {
		t.Errorf("missing roll/accept calls: %+v", reqs)
	}
}

func TestBeerRollVeto(t *testing.T) {
	var reqs []recorded
	var mu sync.Mutex
	srv := newServer(t, &reqs, &mu)
	defer srv.Close()

	// Press 2 → 1 (roll) → 2 (veto).
	sio := &stubIO{keys: []string{"2", "1", "2"}}
	if err := (&Beer{}).Run(context.Background(), sio, map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(sio.said(), "Avvisad") {
		t.Errorf("expected veto confirmation: %q", sio.said())
	}

	mu.Lock()
	defer mu.Unlock()
	var sawVeto bool
	for _, r := range reqs {
		if r.method == http.MethodPost && r.path == "/api/public/roll/103/veto" {
			sawVeto = true
		}
	}
	if !sawVeto {
		t.Errorf("missing veto call: %+v", reqs)
	}
}

func TestBeerBackAtMenu(t *testing.T) {
	var reqs []recorded
	var mu sync.Mutex
	srv := newServer(t, &reqs, &mu)
	defer srv.Close()

	// Press 0 immediately → no BAC/roll calls, just the menu prompt.
	sio := &stubIO{keys: []string{"0"}}
	if err := (&Beer{}).Run(context.Background(), sio, map[string]string{"base_url": srv.URL}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(reqs) != 1 || reqs[0].path != "/api/members" {
		t.Errorf("pressing 0 should only fetch members, got %+v", reqs)
	}
}
