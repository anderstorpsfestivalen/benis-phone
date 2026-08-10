package functions

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/anderstorpsfestivalen/benis-phone/core/bridge"
	"github.com/anderstorpsfestivalen/benis-phone/core/secrets"
)

type RuntimeConfig struct {
	Revision     string              `json:"revision"`
	TOML         string              `json:"toml"`
	SIPPasswords map[string]string   `json:"sip_passwords"`
	Credentials  secrets.Credentials `json:"credentials"`
	Definition   Definition          `json:"-"`
}

// RemoteClient fetches the config bound to one approved bridge identity.
// It never sends or accepts a config name.
type RemoteClient struct {
	BaseURL string
	Signer  *bridge.Signer
	HTTP    *http.Client
}

func NewRemoteClient(baseURL string, signer *bridge.Signer) *RemoteClient {
	return &RemoteClient{
		BaseURL: baseURL,
		Signer:  signer,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (r *RemoteClient) LoadDefinition() (Definition, error) {
	runtime, err := r.LoadRuntimeConfig()
	if err != nil {
		return Definition{}, err
	}
	return runtime.Definition, nil
}

func (r *RemoteClient) LoadRuntimeConfig() (RuntimeConfig, error) {
	body, err := r.fetchPath("/bridge/runtime")
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

func (r *RemoteClient) FetchHash() (string, error) {
	body, err := r.fetchPath("/bridge/hash")
	if err != nil {
		return "", err
	}
	var response struct {
		Revision string `json:"revision"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("decode runtime hash: %w", err)
	}
	if len(response.Revision) != 64 {
		return "", fmt.Errorf("remote returned unexpected revision %q", response.Revision)
	}
	return response.Revision, nil
}

func (r *RemoteClient) SignedRequest(method, path string) (*http.Request, error) {
	u, err := url.Parse(r.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid remote url %q: %w", r.BaseURL, err)
	}
	u.Path = path
	u.RawQuery = ""
	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if r.Signer == nil {
		return nil, fmt.Errorf("remote client has no bridge signer")
	}
	if err := r.Signer.Sign(req, nil); err != nil {
		return nil, err
	}
	return req, nil
}

func (r *RemoteClient) fetchPath(path string) ([]byte, error) {
	req, err := r.SignedRequest(http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote %s returned %s: %s", path, resp.Status, truncate(string(body), 300))
	}
	return body, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
