package tts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// elevenLabsCacheRev bumps whenever the request payload shape changes so
// cached entries from older shapes get invalidated automatically. v1 →
// v2 when we started sending `language_code`.
const elevenLabsCacheRev = "v2"

// ElevenLabs voice IDs go in TTS.Voice. The model_id (e.g.
// "eleven_multilingual_v2", "eleven_turbo_v2_5") goes in TTS.Engine.
// TTS.Language, when set, is forwarded as `language_code` to lock the model
// to a specific language and improve text normalization (per the
// /v1/text-to-speech/{voice_id} API). The Request carries BCP-47 (en-US);
// ElevenLabs wants ISO 639-1 (en), so we strip the region tag below.
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
// The cache revision string bumps whenever we change the payload shape so
// stale entries miss and regenerate automatically.
func (e *elevenLabsProvider) CacheKey(req Request) string {
	model := req.Engine
	if model == "" {
		model = e.defaultModel
	}
	return HashKey("elevenlabs", elevenLabsCacheRev, req.Message, req.Language, req.Voice, model)
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

	payload := map[string]any{
		"text":     req.Message,
		"model_id": model,
	}
	if lang := iso6391(req.Language); lang != "" {
		payload["language_code"] = lang
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s", elevenLabsEndpoint, req.Voice)
	log.WithFields(log.Fields{
		"url":      url,
		"voice":    req.Voice,
		"model":    model,
		"lang_in":  req.Language,
		"lang_api": payload["language_code"],
		"body":     string(body),
	}).Debug("elevenlabs: POST")

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

// iso6391 reduces a BCP-47 tag ("en-US", "sv-SE") to the ISO 639-1 prefix
// ("en", "sv") that the ElevenLabs API expects in `language_code`. Empty
// input returns empty so the field is omitted from the payload.
func iso6391(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	if i := strings.IndexAny(tag, "-_"); i > 0 {
		tag = tag[:i]
	}
	return strings.ToLower(tag)
}
