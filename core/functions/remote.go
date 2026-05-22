package functions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

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
	body, err := r.fetch(false)
	if err != nil {
		return Definition{}, err
	}
	return Decode(body)
}

// FetchHash GETs /config?name=<name>&hash=1 and returns the body trimmed
// of whitespace. The Worker returns sha256(toml) as lowercase hex.
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
	u, err := url.Parse(r.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid remote url %q: %w", r.BaseURL, err)
	}
	u.Path = "/config"
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
		return nil, fmt.Errorf("remote /config returned %s: %s", resp.Status, truncate(string(body), 200))
	}
	return body, nil
}

// LocalHash computes the same SHA-256 hex digest the Worker stores. Used
// for comparing a freshly-loaded local file against the remote hash, and
// for one-shot testing during development.
func LocalHash(toml []byte) string {
	sum := sha256.Sum256(toml)
	return hex.EncodeToString(sum[:])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
