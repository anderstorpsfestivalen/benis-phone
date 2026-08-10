package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/anderstorpsfestivalen/benis-phone/core/secrets"
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

	credentials := secrets.Current()
	username := credentials.Backend.Username
	password := credentials.Backend.Password

	// Error check for missing live runtime credentials.
	if username == "" || password == "" {
		return BalanceResp{}, fmt.Errorf("No credentials for backend loaded.")
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
		return BalanceResp{}, errors.New(br.Message)
	}

	return br, nil

}
