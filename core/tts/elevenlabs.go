package tts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ElevenLabs voice IDs go in TTS.Voice. The model_id (e.g.
// "eleven_multilingual_v2", "eleven_turbo_v2_5") goes in TTS.Engine. Language
// is ignored by ElevenLabs multilingual models but is still part of the cache
// key, so changing it forces a regeneration.
const (
	elevenLabsEndpoint    = "https://api.elevenlabs.io/v1/text-to-speech"
	defaultElevenLabsModel = "eleven_multilingual_v2"
)

type elevenLabsProvider struct {
	apiKey       string
	defaultModel string
	http         *http.Client
}

// NewElevenLabsProvider builds an ElevenLabs Provider. defaultModel is used
// when the per-request Engine is empty (e.g. "eleven_multilingual_v2"); pass
// "" to use the package default.
func NewElevenLabsProvider(apiKey, defaultModel string) Provider {
	if defaultModel == "" {
		defaultModel = defaultElevenLabsModel
	}
	return &elevenLabsProvider{
		apiKey:       apiKey,
		defaultModel: defaultModel,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (e *elevenLabsProvider) Name() string { return "elevenlabs" }

// CacheKey includes the provider name so it can never collide with Polly's
// legacy hash even if the same voice/engine strings are used by accident.
func (e *elevenLabsProvider) CacheKey(req Request) string {
	model := req.Engine
	if model == "" {
		model = e.defaultModel
	}
	return HashKey("elevenlabs", req.Message, req.Language, req.Voice, model)
}

func (e *elevenLabsProvider) Synthesize(req Request) ([]byte, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("elevenlabs: missing API key")
	}
	if req.Voice == "" {
		return nil, fmt.Errorf("elevenlabs: missing voice ID (set tts.voice to a voice_id)")
	}

	model := req.Engine
	if model == "" {
		model = e.defaultModel
	}

	body, err := json.Marshal(map[string]any{
		"text":     req.Message,
		"model_id": model,
	})
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s", elevenLabsEndpoint, req.Voice)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("xi-api-key", e.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "audio/mpeg")

	resp, err := e.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("elevenlabs: %s: %s", resp.Status, string(msg))
	}

	return io.ReadAll(resp.Body)
}
