package functions

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type RuntimeConfig struct {
	Revision     string            `json:"revision"`
	TOML         string            `json:"toml"`
	SIPPasswords map[string]string `json:"sip_passwords"`
	Definition   Definition        `json:"-"`
}

// RemoteClient fetches configs from the Cloudflare Worker (pbx.<zone>/config).
type RemoteClient struct {
	BaseURL string // e.g. "https://ivr.anderstorpsfestivalen.se"
	Name    string // config name, e.g. "simonstorp"
	Token   string // bearer token (creds/creds.json PBXConfigToken)

	HTTP *http.Client
}

// NewRemoteClient builds a client with a sane HTTP timeout.
func NewRemoteClient(baseURL, name, token string) *RemoteClient {
	return &RemoteClient{
		BaseURL: baseURL,
		Name:    name,
		Token:   token,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// LoadDefinition GETs /config?name=<name>, decodes the TOML, and returns
// the prepared Definition. Errors carry HTTP status when relevant.
func (r *RemoteClient) LoadDefinition() (Definition, error) {
	runtime, err := r.LoadRuntimeConfig()
	if err != nil {
		return Definition{}, err
	}
	return runtime.Definition, nil
}

// LoadRuntimeConfig fetches the editor-safe TOML together with the SIP
// passwords that only the bearer-authenticated IVR runtime is allowed to see.
func (r *RemoteClient) LoadRuntimeConfig() (RuntimeConfig, error) {
	body, err := r.fetchPath("/config/runtime", false)
	if err != nil {
		return RuntimeConfig{}, err
	}
	var cfg RuntimeConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return RuntimeConfig{}, fmt.Errorf("decode runtime config: %w", err)
	}
	def, err := Decode([]byte(cfg.TOML))
	if err != nil {
		return RuntimeConfig{}, err
	}
	cfg.Definition = def
	if cfg.SIPPasswords == nil {
		cfg.SIPPasswords = make(map[string]string)
	}
	return cfg, nil
}

// FetchHash GETs /config?name=<name>&hash=1 and returns the opaque revision
// digest, which changes for either TOML or encrypted-secret updates.
func (r *RemoteClient) FetchHash() (string, error) {
	body, err := r.fetch(true)
	if err != nil {
		return "", err
	}
	h := string(body)
	// Trim trailing newlines/spaces that an HTTP intermediary might add.
	for len(h) > 0 && (h[len(h)-1] == '\n' || h[len(h)-1] == '\r' || h[len(h)-1] == ' ') {
		h = h[:len(h)-1]
	}
	if len(h) != 64 {
		return "", fmt.Errorf("remote returned unexpected hash %q (want 64-char hex)", h)
	}
	return h, nil
}

func (r *RemoteClient) fetch(hashOnly bool) ([]byte, error) {
	return r.fetchPath("/config", hashOnly)
}

func (r *RemoteClient) fetchPath(path string, hashOnly bool) ([]byte, error) {
	u, err := url.Parse(r.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid remote url %q: %w", r.BaseURL, err)
	}
	u.Path = path
	q := u.Query()
	q.Set("name", r.Name)
	if hashOnly {
		q.Set("hash", "1")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if r.Token != "" {
		req.Header.Set("Authorization", "Bearer "+r.Token)
	}

	resp, err := r.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote %s returned %s: %s", path, resp.Status, truncate(string(body), 200))
	}
	// Sanity check: TOML never starts with '<'. If it does, we got an HTML
	// page back through a 200 response — usually Cloudflare Access serving
	// its login page because the bearer-token bypass didn't match. Surface
	// that instead of letting the TOML parser fail with a cryptic error.
	trimmed := body
	for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\n' || trimmed[0] == '\r' || trimmed[0] == '\t') {
		trimmed = trimmed[1:]
	}
	if len(trimmed) > 0 && trimmed[0] == '<' {
		ctype := resp.Header.Get("Content-Type")
		return nil, fmt.Errorf(
			"remote /config returned HTML (Content-Type=%q) on a 200 response — "+
				"the bearer-token bypass for /config* is probably not applied to your Access policy, "+
				"or PBXConfigToken doesn't match the worker's CONFIG_BEARER_TOKEN secret. "+
				"Body preview: %s",
			ctype, truncate(string(body), 200),
		)
	}
	return body, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
