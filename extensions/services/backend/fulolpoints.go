package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/anderstorpsfestivalen/benis-phone/core/secrets"
)

type FulolPointsResp struct {
	Points  float64 `json:"Points"`
	Name    string  `json:"Name"`
	Message string  `json:"Message"`
}

func GetFulolPointsForPhoneNumber(number string) (FulolPointsResp, error) {

	br := FulolPointsResp{}

	client := &http.Client{}
	form := url.Values{}
	form.Set("number", number)
	fmt.Println("Inputted number is: " + number)

	req, err := http.NewRequest("POST", "https://anderstorpsfestivalen.se/api/phone/fulolpoints", strings.NewReader(form.Encode()))
	if err != nil {
		return FulolPointsResp{}, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(form.Encode())))

	username := secrets.Loaded.Backend.Username
	password := secrets.Loaded.Backend.Password

	// Error check for missing credentials in creds.json
	if secrets.Loaded.Backend.Username == "" || secrets.Loaded.Backend.Password == "" {
		return FulolPointsResp{}, fmt.Errorf("No credentials for backend loaded.")
	}

	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		return FulolPointsResp{}, err
	}

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&br)
	if err != nil {
		return FulolPointsResp{}, err
	}

	if br.Message != "" {
		return FulolPointsResp{}, fmt.Errorf(br.Message)
	}

	return br, nil

}
