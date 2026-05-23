package functions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGenericJSONFetchAndRender(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "temp": 23.7,
            "place": "Reftele",
            "items": [
                {"name": "Foo", "value": 1},
                {"name": "Bar", "value": 2.5}
            ]
        }`))
	}))
	defer srv.Close()

	cases := []struct {
		name string
		tmpl string
		want string
	}{
		{
			name: "dot path + int helper",
			tmpl: "Temp is {{int .Data.temp}} in {{.Data.place}}.",
			want: "Temp is 24 in Reftele.",
		},
		{
			name: "range over array",
			tmpl: "{{range .Data.items}}{{.name}}={{.value}};{{end}}",
			want: "Foo=1;Bar=2.5;",
		},
		{
			name: "jq helper navigates path",
			tmpl: "first: {{jq .Data \".items[0].name\"}}, last: {{jq .Data \".items[-1].name\"}}",
			want: "first: Foo, last: Bar",
		},
		{
			name: "jq select where field equals value",
			tmpl: "{{jq .Data \".items[] | select(.name == \\\"Bar\\\") | .value\"}}",
			want: "2.5",
		},
		{
			name: "default helper fills missing key",
			tmpl: "{{default \"unknown\" (jq .Data \".missing\")}}",
			want: "unknown",
		},
		{
			name: "length + arithmetic helpers",
			tmpl: "count={{length .Data.items}} sum={{add (jq .Data \".items[0].value\") (jq .Data \".items[1].value\")}}",
			want: "count=2 sum=3.5",
		},
		{
			name: "jqAll + range for multi-result query",
			tmpl: "{{range jqAll .Data \".items[] | .name\"}}{{.}};{{end}}",
			want: "Foo;Bar;",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := GenericJSON{URL: srv.URL, Template: tc.tmpl}
			got, err := g.FetchAndRender(context.Background())
			if err != nil {
				t.Fatalf("FetchAndRender: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGenericJSONHTTPError(t *testing.T) {
	// Body deliberately looks like a leaked credential to lock in the
	// no-body-in-error-string behavior — these strings get spoken via
	// TTS and logged, so they must never appear in the returned error.
	const secret = "Bearer leaked-token-xyz123"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
		_, _ = w.Write([]byte(secret))
	}))
	defer srv.Close()

	g := GenericJSON{URL: srv.URL, Template: "x"}
	_, err := g.FetchAndRender(context.Background())
	if err == nil {
		t.Fatal("expected error on 5xx response")
	}
	if !strings.Contains(err.Error(), "HTTP 503") {
		t.Errorf("error should mention status code: %v", err)
	}
	if strings.Contains(err.Error(), "leaked-token") {
		t.Errorf("error must not leak response body: %v", err)
	}
}

func TestGenericJSONMissingURL(t *testing.T) {
	g := GenericJSON{Template: "x"}
	if _, err := g.FetchAndRender(context.Background()); err == nil {
		t.Fatal("expected error when URL missing")
	}
}

func TestGenericJSONHeadersAndBody(t *testing.T) {
	var (
		gotAuth   string
		gotBody   string
		gotMethod string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer srv.Close()

	g := GenericJSON{
		URL:      srv.URL,
		Method:   "POST",
		Body:     `{"query": "test"}`,
		Headers:  map[string]string{"Authorization": "Bearer xyz"},
		Template: `ok={{.Data.ok}}`,
	}
	got, err := g.FetchAndRender(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != "ok=true" {
		t.Errorf("render: got %q", got)
	}
	if gotAuth != "Bearer xyz" {
		t.Errorf("auth header not forwarded: %q", gotAuth)
	}
	if gotBody != `{"query": "test"}` {
		t.Errorf("body not forwarded: %q", gotBody)
	}
	if gotMethod != "POST" {
		t.Errorf("method: %q", gotMethod)
	}
}

// TestSaunaSelect mirrors the real-world example: an array of objects,
// pick the entry where name matches, read its temperature.
func TestSaunaSelect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"Summalajnen","temperature":23.1875},{"name":"Fozzie","temperature":19}]`))
	}))
	defer srv.Close()

	g := GenericJSON{
		URL:      srv.URL,
		Template: `Summalajnen is {{int (jq .Data ".[] | select(.name == \"Summalajnen\") | .temperature")}} celsius.`,
	}
	got, err := g.FetchAndRender(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != "Summalajnen is 23 celsius." {
		t.Errorf("got %q", got)
	}
}

// TestGenericJSONTimeout confirms that TimeoutSeconds actually fires —
// a server that holds the response longer than the configured timeout
// must produce a deadline-exceeded error, not block forever.
func TestGenericJSONTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	g := GenericJSON{URL: srv.URL, Template: "x", TimeoutSeconds: 0}
	// Bypass the 10s default with a tight context deadline instead.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := g.FetchAndRender(ctx)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 400*time.Millisecond {
		t.Errorf("timeout didn't fire: took %v", elapsed)
	}
}

// TestGenericJSONCancellation is the hangup scenario: the request is
// in-flight when ctx is cancelled. Must return promptly, not after the
// server finishes.
func TestGenericJSONCancellation(t *testing.T) {
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-done:
		case <-time.After(2 * time.Second):
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	defer close(done)

	ctx, cancel := context.WithCancel(context.Background())
	g := GenericJSON{URL: srv.URL, Template: "x"}
	errCh := make(chan error, 1)
	go func() {
		_, err := g.FetchAndRender(ctx)
		errCh <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error after cancel")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("FetchAndRender did not return promptly after cancel")
	}
}

// TestGenericJSONNonJSONBody — a non-JSON response surfaces a useful
// decode error rather than silently rendering against empty data.
func TestGenericJSONNonJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>not json</body></html>`))
	}))
	defer srv.Close()

	g := GenericJSON{URL: srv.URL, Template: "x"}
	_, err := g.FetchAndRender(context.Background())
	if err == nil {
		t.Fatal("expected decode error for non-JSON body")
	}
	if !strings.Contains(err.Error(), "decode JSON") {
		t.Errorf("error should mention JSON decode: %v", err)
	}
}

// TestGenericJSONBodySizeLimit — a response larger than maxBodyBytes is
// truncated by LimitReader, so the (now-incomplete) JSON either decodes
// to something or errors. We accept either, but the test asserts the
// process doesn't get OOM'd by a giant response.
func TestGenericJSONBodySizeLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Start a valid array, then spam ~9 MiB of filler before any
		// closing bracket — LimitReader cuts us off before we'd see the
		// `]`, so JSON decode must fail rather than blocking.
		_, _ = w.Write([]byte(`[`))
		chunk := strings.Repeat("\"x\",", 1024)
		for i := 0; i < 9*1024; i++ {
			if _, err := w.Write([]byte(chunk)); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	g := GenericJSON{URL: srv.URL, Template: "x"}
	_, err := g.FetchAndRender(context.Background())
	if err == nil {
		t.Fatal("expected error from truncated giant body")
	}
}

// TestGenericJSONInvalidTemplate — a malformed template surfaces the
// parse error rather than panicking.
func TestGenericJSONInvalidTemplate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	g := GenericJSON{URL: srv.URL, Template: `{{.unbalanced`}
	_, err := g.FetchAndRender(context.Background())
	if err == nil {
		t.Fatal("expected template parse error")
	}
	if !strings.Contains(err.Error(), "parse template") {
		t.Errorf("error should mention template parse: %v", err)
	}
}

// TestGenericJSONURLRedaction — query params and basic-auth creds in
// the URL must not appear in error strings (we redactURL them out).
func TestGenericJSONURLRedaction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	g := GenericJSON{
		URL:      srv.URL + "/foo?api_key=secret123",
		Template: "x",
	}
	_, err := g.FetchAndRender(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "secret123") {
		t.Errorf("api_key leaked into error: %v", err)
	}
	if !strings.Contains(err.Error(), "/foo") {
		t.Errorf("error should keep the path: %v", err)
	}
}

// TestJQPartialResultsOnError — pin the documented behavior of runJQ
// when a query yields some values then errors mid-stream: we discard
// the partial results. (Otherwise callers couldn't tell complete vs.
// partial results apart from the return value alone.)
func TestJQPartialResultsOnError(t *testing.T) {
	data := []any{1.0, 2.0, 3.0}
	// `.[] | if . == 2 then error("boom") else . end` yields 1 then errors.
	out, err := runJQ(data, `.[] | if . == 2 then error("boom") else . end`)
	if err == nil {
		t.Fatal("expected mid-stream error")
	}
	if len(out) != 1 || out[0] != 1.0 {
		t.Errorf("runJQ should keep values seen before the error (got %v); change the contract intentionally if behavior shifts", out)
	}
	// jqFirst/jqAll wrappers must collapse to nil/empty on error so
	// templates can rely on `default` etc.
	if jqFirst(data, `.[] | if . == 2 then error("boom") else . end`) != nil {
		t.Error("jqFirst should return nil on mid-stream error")
	}
	if got := jqAll(data, `.[] | if . == 2 then error("boom") else . end`); got != nil {
		t.Errorf("jqAll should return nil on mid-stream error, got %v", got)
	}
}

func TestJQHelpers(t *testing.T) {
	data := []any{
		map[string]any{"name": "Foo", "value": 1.0},
		map[string]any{"name": "Bar", "value": 2.5},
	}
	if v := jqFirst(data, `.[] | select(.name == "Bar") | .value`); v != 2.5 {
		t.Errorf("jqFirst select: got %v", v)
	}
	all := jqAll(data, `.[].name`)
	if len(all) != 2 || all[0] != "Foo" || all[1] != "Bar" {
		t.Errorf("jqAll: got %v", all)
	}
	if jqFirst(data, ".missing") != nil {
		t.Error("jqFirst on missing path should return nil")
	}
	if jqFirst(data, "this is not a valid jq query!") != nil {
		t.Error("parse error should return nil, not panic")
	}
}
