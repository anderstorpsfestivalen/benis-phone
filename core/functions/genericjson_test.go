package functions

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
			got, err := g.FetchAndRender()
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`server down`))
	}))
	defer srv.Close()

	g := GenericJSON{URL: srv.URL, Template: "x"}
	_, err := g.FetchAndRender()
	if err == nil {
		t.Fatal("expected error on 5xx response")
	}
	if !strings.Contains(err.Error(), "HTTP 503") {
		t.Errorf("error should mention status code: %v", err)
	}
}

func TestGenericJSONMissingURL(t *testing.T) {
	g := GenericJSON{Template: "x"}
	if _, err := g.FetchAndRender(); err == nil {
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
	got, err := g.FetchAndRender()
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
	got, err := g.FetchAndRender()
	if err != nil {
		t.Fatal(err)
	}
	if got != "Summalajnen is 23 celsius." {
		t.Errorf("got %q", got)
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
