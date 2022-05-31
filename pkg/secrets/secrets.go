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
