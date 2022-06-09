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

	req, err := http.NewRequest("GET", "https://www.systembolaget.se/", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authority", "www.systembolaget.se")
	req.Header.Set("Sec-Ch-Ua", "\"Chromium\";v=\"88\", \"Google Chrome\";v=\"88\", \";Not A Brand\";v=\"99\"")
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Dnt", "1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.192 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,sv;q=0.8")

	resp, err := http.DefaultClient.Do(req)
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

	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Sec-Ch-Ua", "\"Chromium\";v=\"88\", \"Google Chrome\";v=\"88\", \";Not A Brand\";v=\"99\"")
	req.Header.Set("Referer", "https://www.systembolaget.se/")
	req.Header.Set("Dnt", "1")
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.192 Safari/537.36")

	resp, err := http.DefaultClient.Do(req)
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
