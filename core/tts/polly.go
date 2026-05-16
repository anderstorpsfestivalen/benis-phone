package tts

import (
	"github.com/anderstorpsfestivalen/benis-phone/core/polly"
)

// pollyProvider adapts the existing polly.Polly client to the Provider
// interface. CacheKey matches Polly's legacy hash format so the existing
// haschcache directory stays valid across this refactor.
type pollyProvider struct {
	client polly.Polly
}

// NewPollyProvider wraps an existing polly.Polly client.
func NewPollyProvider(client polly.Polly) Provider {
	return &pollyProvider{client: client}
}

func (p *pollyProvider) Name() string { return "polly" }

// CacheKey matches polly.Polly.haschRequest: sha1(message + language + voice + engine).
// Keeping the legacy key means the prewarmed haschcache stays useful.
func (p *pollyProvider) CacheKey(req Request) string {
	return HashKey(req.Message, req.Language, req.Voice, req.Engine)
}

func (p *pollyProvider) Synthesize(req Request) ([]byte, error) {
	return p.client.TTSLang(req.Message, req.Language, req.Voice, req.Engine)
}
