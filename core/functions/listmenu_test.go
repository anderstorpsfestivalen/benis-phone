package functions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListMenuBuild(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":13,"name":"alexfoo"},{"id":11,"name":"coral"},{"id":12,"name":"wberg"}]`))
	}))
	defer srv.Close()

	m := ListMenu{
		URL:   srv.URL,
		List:  "sort_by(.id)",
		Label: "{{.name}}",
		Store: "member",
		Dst:   "beer_bac",
	}
	items, prompt, err := m.Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// sort_by(.id): coral(11), wberg(12), alexfoo(13).
	if len(items) != 3 {
		t.Fatalf("want 3 items, got %d", len(items))
	}
	first, _ := items[0].(map[string]any)
	if first["name"] != "coral" {
		t.Errorf("items not id-sorted: %v", items)
	}
	want := "Tryck 1 för coral. Tryck 2 för wberg. Tryck 3 för alexfoo. "
	if prompt != want {
		t.Errorf("prompt:\n got %q\nwant %q", prompt, want)
	}
}

func TestListMenuMaxCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"n":"a"},{"n":"b"},{"n":"c"},{"n":"d"}]`))
	}))
	defer srv.Close()

	m := ListMenu{URL: srv.URL, Label: "{{.n}}", Max: 2}
	items, _, err := m.Build(context.Background(), nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("Max=2 not honored: got %d items", len(items))
	}
}
