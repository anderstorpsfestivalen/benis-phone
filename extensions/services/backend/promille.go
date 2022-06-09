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

type PromilleResp struct {
	Promille float64 `json:"Promille"`
	Name     string  `json:"Name"`
	Message  string  `json:"Message"`
}

func GetPromilleForPhoneNumber(number string) (PromilleResp, error) {

	pr := PromilleResp{}

	client := &http.Client{}
	form := url.Values{}
	form.Set("number", number)
	fmt.Println("Inputted number is: " + number)

	req, err := http.NewRequest("POST", "https://anderstorpsfestivalen.se/api/phone/promille", strings.NewReader(form.Encode()))
	if err != nil {
		return PromilleResp{}, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(form.Encode())))

	username := secrets.Loaded.Backend.Username
	password := secrets.Loaded.Backend.Password

	// Error check for missing credentials in creds.json
	if secrets.Loaded.Backend.Username == "" || secrets.Loaded.Backend.Password == "" {
		return PromilleResp{}, fmt.Errorf("No credentials for backend loaded.")
	}

	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		return PromilleResp{}, err
	}

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&pr)
	if err != nil {
		return PromilleResp{}, err
	}

	if resp.StatusCode == 400 {
		return PromilleResp{}, fmt.Errorf("no transactions")
	}

	if pr.Message != "" {
		return PromilleResp{}, fmt.Errorf(pr.Message)
	}

	return pr, nil

}
