package systemet

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

type SubKeyResp struct {
	APIGatewayEndpoint          string   `json:"apiGatewayEndpoint"`
	OcpApimSubscriptionKey      string   `json:"ocpApimSubscriptionKey"`
	APIGatewayVersion           string   `json:"apiGatewayVersion"`
	BasketMaxItems              int      `json:"basketMaxItems"`
	BasketRowMaxItems           int      `json:"basketRowMaxItems"`
	BasketCookieDomain          string   `json:"basketCookieDomain"`
	MaxStockBalance             int      `json:"maxStockBalance"`
	ParcelSlots                 int      `json:"parcelSlots"`
	AdyenOriginKey              string   `json:"adyenOriginKey"`
	AdyenJsEndpoint             string   `json:"adyenJsEndpoint"`
	AdyenShaIntegrity           string   `json:"adyenShaIntegrity"`
	AdyenEnvironment            string   `json:"adyenEnvironment"`
	AdyenLoadingContextEndpoint string   `json:"adyenLoadingContextEndpoint"`
	LoginURL                    string   `json:"loginUrl"`
	LogoutURL                   string   `json:"logoutUrl"`
	CdnExternalMediaURL         string   `json:"cdnExternalMediaUrl"`
	GoogleAPIKey                string   `json:"googleApiKey"`
	SearchableFilters           []string `json:"searchableFilters"`
	AdyenLiveTest               struct {
		AdyenOriginKey              interface{} `json:"adyenOriginKey"`
		AdyenJsEndpoint             interface{} `json:"adyenJsEndpoint"`
		AdyenShaIntegrity           interface{} `json:"adyenShaIntegrity"`
		AdyenEnvironment            interface{} `json:"adyenEnvironment"`
		AdyenLoadingContextEndpoint interface{} `json:"adyenLoadingContextEndpoint"`
	} `json:"adyenLiveTest"`
}

func GetKey() (string, error) {
	settingsPathEsc, err := getAppSettingsPath()
	if err != nil {
		return "", err
	}

	settingsPath, err := extractSrcFromSettingsPath(settingsPathEsc)
	if err != nil {
		return "", err
	}

	key, err := getSettings(settingsPath)
	if err != nil {
		return "", err
	}

	return key, nil

}

func getAppSettingsPath() (string, error) {

	resp, err := http.Get("https://systembolaget.se")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "appsettings") {
			return scanner.Text(), nil
		}

	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", err
}

func extractSrcFromSettingsPath(s string) (string, error) {

	re := regexp.MustCompile(`<script[^>]+\bsrc=["']([^"']+)["']`)

	submatchall := re.FindAllStringSubmatch(s, -1)
	for _, element := range submatchall {
		return element[1], nil
	}

	return "", fmt.Errorf("Could not find app settings path string")
}

func getSettings(path string) (string, error) {
	resp, err := http.Get(path)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ss := string(body)
	ss = strings.Replace(ss, "window.appSettings = Object.freeze(", "", -1)
	ss = strings.Replace(ss, "})", "}", -1)

	var skey SubKeyResp
	err = json.Unmarshal([]byte(ss), &skey)
	if err != nil {
		return "", err
	}

	return skey.OcpApimSubscriptionKey, nil
}
