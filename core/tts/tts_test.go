package tts

import (
	"fmt"
	"sync"
	"testing"
)

type blockingProvider struct {
	name    string
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (provider *blockingProvider) Name() string { return provider.name }
func (provider *blockingProvider) CacheKey(Request) string {
	return provider.name
}
func (provider *blockingProvider) Synthesize(Request) ([]byte, error) {
	provider.once.Do(func() { close(provider.started) })
	<-provider.release
	return []byte(provider.name), nil
}

type immediateProvider string

func (provider immediateProvider) Name() string            { return string(provider) }
func (provider immediateProvider) CacheKey(Request) string { return string(provider) }
func (provider immediateProvider) Synthesize(Request) ([]byte, error) {
	return []byte(provider), nil
}

type countingProvider struct {
	mu    sync.Mutex
	calls int
}

func (provider *countingProvider) Name() string                { return "counting" }
func (provider *countingProvider) CacheKey(req Request) string { return HashKey(req.Message) }
func (provider *countingProvider) Synthesize(Request) ([]byte, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.calls++
	return []byte(fmt.Sprintf("call-%d", provider.calls)), nil
}

func (provider *countingProvider) callCount() int {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.calls
}

func TestRegistryReplacementPreservesInFlightSynthesis(t *testing.T) {
	old := &blockingProvider{
		name:    "old",
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	registry := NewRegistry(t.TempDir(), "old")
	if err := registry.Replace("old", old); err != nil {
		t.Fatal(err)
	}
	result := make(chan string, 1)
	go func() {
		data, err := registry.Synthesize("", Request{Message: "during-reload"})
		if err != nil {
			result <- "error:" + err.Error()
			return
		}
		result <- string(data)
	}()
	<-old.started
	if err := registry.Replace("new", immediateProvider("new")); err != nil {
		t.Fatal(err)
	}
	close(old.release)
	if got := <-result; got != "old" {
		t.Fatalf("in-flight synthesis = %q, want old provider completion", got)
	}
	data, err := registry.Synthesize("", Request{Message: "after-reload"})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("post-reload synthesis = %q", data)
	}
}

func TestRegistryCachesByDefaultAndCanBypass(t *testing.T) {
	provider := &countingProvider{}
	registry := NewRegistry(t.TempDir(), provider.Name())
	registry.Register(provider)

	first, err := registry.Synthesize("", Request{Message: "menu"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.Synthesize("", Request{Message: "menu"})
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != "call-1" || string(second) != "call-1" {
		t.Fatalf("cached results = %q, %q; want call-1 twice", first, second)
	}
	if got := provider.callCount(); got != 1 {
		t.Fatalf("provider calls after cached requests = %d, want 1", got)
	}

	third, err := registry.Synthesize("", Request{Message: "menu", NoCache: true})
	if err != nil {
		t.Fatal(err)
	}
	fourth, err := registry.Synthesize("", Request{Message: "menu", NoCache: true})
	if err != nil {
		t.Fatal(err)
	}
	if string(third) != "call-2" || string(fourth) != "call-3" {
		t.Fatalf("uncached results = %q, %q; want call-2, call-3", third, fourth)
	}
	if got := provider.callCount(); got != 3 {
		t.Fatalf("provider calls after bypassed requests = %d, want 3", got)
	}

	// A bypassed request must not overwrite the existing cached menu audio.
	again, err := registry.Synthesize("", Request{Message: "menu"})
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != "call-1" {
		t.Fatalf("cached result after bypass = %q, want call-1", again)
	}
}
