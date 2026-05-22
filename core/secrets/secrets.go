package secrets

import (
	"encoding/json"
	"io/ioutil"
)

type AWSCred struct {
	Key    string
	Secret string
}

type PWCombo struct {
	Username string
	Password string
}

type Credentials struct {
	S3        AWSCred
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
