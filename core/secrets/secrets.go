package secrets

import (
	"sync/atomic"
)

type AWSCred struct {
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

// R2Cred carries the S3-compatible credentials for Cloudflare R2. Both keys
// come from the dashboard's "R2 API Tokens" / "Account API Tokens with R2
// permissions" creation page. AccountID is the Cloudflare account UUID
// (used to derive the S3 endpoint host).
type R2Cred struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	AccountID       string `json:"account_id"`
	Bucket          string `json:"bucket"`
}

type PWCombo struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Credentials struct {
	R2      R2Cred  `json:"r2"`
	Polly   AWSCred `json:"polly"`
	Backend PWCombo `json:"backend"`

	Trafikverket string `json:"trafikverket_key"`

	HTTPServerAuth PWCombo `json:"http_server_auth"`
	MediaServer    string  `json:"media_server_url"`

	// ElevenLabs API key (single-key auth). Optional; provider is only
	// registered when a non-empty value is present.
	ElevenLabs string `json:"elevenlabs_api_key"`
}

var loaded atomic.Pointer[Credentials]

func Replace(credentials Credentials) {
	copy := credentials
	loaded.Store(&copy)
}

func Current() Credentials {
	credentials := loaded.Load()
	if credentials == nil {
		return Credentials{}
	}
	return *credentials
}
