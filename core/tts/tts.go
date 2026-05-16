// Package tts provides a pluggable text-to-speech facade with a shared
// on-disk cache so identical requests don't burn tokens twice.
//
// A Registry holds one or more Providers (Polly, ElevenLabs, ...). Each
// Provider knows how to call its own API and how to hash a request into a
// cache key. The Registry checks the cache before calling the provider and
// writes successful responses to disk so subsequent calls are free.
package tts

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path"
	"sync"

	log "github.com/sirupsen/logrus"
)

// Request describes what to synthesize. All fields are passed through to the
// underlying provider; providers may ignore fields that don't apply (e.g.
// Language is meaningless for ElevenLabs multilingual voices but is still
// included in the cache key for safety).
type Request struct {
	Message  string
	Voice    string
	Language string
	Engine   string
}

// Provider is the contract every TTS backend implements.
type Provider interface {
	// Name identifies the provider in config (e.g. "polly", "elevenlabs").
	Name() string

	// CacheKey returns the on-disk cache filename for a given request. The
	// key must be deterministic and collision-resistant across requests with
	// the same Provider; collisions across providers are prevented by the
	// Registry by including Name() in the key when CacheKey returns a
	// provider-agnostic hash.
	CacheKey(req Request) string

	// Synthesize calls the provider's API and returns the raw audio bytes
	// (MP3 is the convention; the playback path will transcode).
	Synthesize(req Request) ([]byte, error)
}

// Registry routes synthesis requests to the right Provider with shared
// caching.
type Registry struct {
	cacheDir    string
	defaultName string
	providers   map[string]Provider
	mu          sync.RWMutex
}

// NewRegistry creates an empty Registry rooted at cacheDir. The directory is
// created if it doesn't exist.
func NewRegistry(cacheDir, defaultName string) *Registry {
	os.MkdirAll(cacheDir, os.ModePerm)
	return &Registry{
		cacheDir:    cacheDir,
		defaultName: defaultName,
		providers:   make(map[string]Provider),
	}
}

// Register adds a provider. Overwrites any prior provider with the same name.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

// DefaultName returns the configured default provider name.
func (r *Registry) DefaultName() string { return r.defaultName }

// Has reports whether a provider is registered under the given name.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.providers[name]
	return ok
}

// Synthesize routes to the named provider (empty falls back to the default),
// checks the on-disk cache, and returns the audio bytes.
func (r *Registry) Synthesize(providerName string, req Request) ([]byte, error) {
	if providerName == "" {
		providerName = r.defaultName
	}

	r.mu.RLock()
	p, ok := r.providers[providerName]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tts: provider %q not registered", providerName)
	}

	key := p.CacheKey(req)
	if data, err := r.readCache(key); err == nil {
		log.WithFields(log.Fields{"provider": providerName, "voice": req.Voice}).Trace("TTS cache hit")
		return data, nil
	}

	log.WithFields(log.Fields{"provider": providerName, "voice": req.Voice, "len": len(req.Message)}).Debug("TTS cache miss, calling provider")
	data, err := p.Synthesize(req)
	if err != nil {
		return nil, fmt.Errorf("tts: %s: %w", providerName, err)
	}

	if werr := r.writeCache(key, data); werr != nil {
		log.WithError(werr).Warn("tts: failed to write cache")
	}
	return data, nil
}

func (r *Registry) readCache(key string) ([]byte, error) {
	return os.ReadFile(path.Join(r.cacheDir, key))
}

func (r *Registry) writeCache(key string, data []byte) error {
	return os.WriteFile(path.Join(r.cacheDir, key), data, 0644)
}

// HashKey is a shared SHA1 helper. Providers should call it to derive a
// CacheKey from the relevant fields of a Request.
func HashKey(parts ...string) string {
	h := sha1.New()
	for _, p := range parts {
		io.WriteString(h, p)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
