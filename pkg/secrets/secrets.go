package secrets

import (
	"encoding/json"
	"io/ioutil"
)

type AWSCred struct {
	Key    string
	Secret string
}

type Credentials struct {
	S3        AWSCred
	Polly     AWSCred
	Trafiklab string
}

func LoadSecrets() Credentials {

	var c Credentials
	data, err := ioutil.ReadFile("./creds/creds.json")
	if err != nil {
		panic("Could not load credentials")
	}

	json.Unmarshal(data, &c)

	return c
}
