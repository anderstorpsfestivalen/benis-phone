package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/secrets"
)

type BalanceResp struct {
	Balance float64 `json:"Balance"`
	Name    string  `json:"Name"`
	Message string  `json:"Message"`
}

func GetBalanceForPhoneNumber(number string) (BalanceResp, error) {

	br := BalanceResp{}

	client := &http.Client{}
	form := url.Values{}
	form.Set("number", number)
	fmt.Println("Inputted number is: " + number)

	req, err := http.NewRequest("POST", "https://anderstorpsfestivalen.se/api/phone/balance", strings.NewReader(form.Encode()))
	if err != nil {
		return BalanceResp{}, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(form.Encode())))

	username := secrets.Loaded.Backend.Username
	password := secrets.Loaded.Backend.Password

	// Error check for missing credentials in creds.json
	if secrets.Loaded.Backend.Username == "" || secrets.Loaded.Backend.Password == "" {
		return BalanceResp{}, fmt.Errorf("No credentials for admin loaded.")
	}

	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		return BalanceResp{}, err
	}

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&br)
	if err != nil {
		return BalanceResp{}, err
	}

	if br.Message != "" {
		return BalanceResp{}, fmt.Errorf(br.Message)
	}

	return br, nil

}
