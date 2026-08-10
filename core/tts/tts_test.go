package tts

import (
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
