package secrets

import (
	"encoding/json"
	"io/ioutil"
)

type AWSCred struct {
	Key    string
	Secret string
}

// R2Cred carries the S3-compatible credentials for Cloudflare R2. Both keys
// come from the dashboard's "R2 API Tokens" / "Account API Tokens with R2
// permissions" creation page. AccountID is the Cloudflare account UUID
// (used to derive the S3 endpoint host).
type R2Cred struct {
	AccessKeyID     string
	SecretAccessKey string
	AccountID       string
	Bucket          string
}

type PWCombo struct {
	Username string
	Password string
}

type Credentials struct {
	S3        AWSCred
	R2        R2Cred
	Polly     AWSCred
	Backend   PWCombo
	Trafiklab string
	Systemet  string

	HTTPServerAuth PWCombo
	MediaServer    string

	// SIP credentials for SIP trunk authentication
	SIP PWCombo

	// ElevenLabs API key (single-key auth). Optional; provider is only
	// registered when a non-empty value is present.
	ElevenLabs string

	// PBXConfigToken is the bearer token the binary sends to the remote
	// config worker (pbx.<zone>/config) when -source=remote. Required for
	// remote mode; ignored otherwise.
	PBXConfigToken string
}

var Loaded Credentials

func LoadSecrets() (Credentials, error) {

	var c Credentials
	data, err := ioutil.ReadFile("./creds/creds.json")
	if err != nil {
		return Credentials{}, err
	}

	json.Unmarshal(data, &c)

	Loaded = c

	return c, nil
}
