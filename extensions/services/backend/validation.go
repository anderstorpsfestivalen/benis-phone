package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"gitlab.com/anderstorpsfestivalen/benis-phone/core/secrets"
)

type ValidResp struct {
	Valid   bool   `json:"Name"`
	Message string `json:"Message"`
}

func CheckValidNumber(number string) (bool, error) {

	br := ValidResp{}

	client := &http.Client{}
	form := url.Values{}
	form.Set("number", number)

	req, err := http.NewRequest("POST", "https://anderstorpsfestivalen.se/api/phone/balance", strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(form.Encode())))

	username := secrets.Loaded.Backend.Username
	password := secrets.Loaded.Backend.Password

	// Error check for missing credentials in creds.json
	if secrets.Loaded.Backend.Username == "" || secrets.Loaded.Backend.Password == "" {
		return false, fmt.Errorf("no credentials for backend loaded")
	}

	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&br)
	if err != nil {
		return false, err
	}

	return br.Valid, nil

}
