package systemet

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type StockResponse []struct {
	ProductID string `json:"productId"`
	StoreID   string `json:"storeId"`
	Shelf     string `json:"shelf"`
	Stock     int    `json:"stock"`
}

type SystemetV2 struct {
	key string
}

func New(key string) *SystemetV2 {
	return &SystemetV2{
		key: key,
	}
}

func (s *SystemetV2) GetStock(productID string, storeID string) (StockResponse, error) {

	req, err := http.NewRequest("GET", "https://api-extern.systembolaget.se/sb-api-ecommerce/v1/stockbalance/store?ProductId="+productID+"&StoreId="+storeID, nil)
	if err != nil {
		// handle err
	}
	req.Header.Set("Authority", "api-extern.systembolaget.se")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Sec-Ch-Ua", "\"Chromium\";v=\"88\", \"Google Chrome\";v=\"88\", \";Not A Brand\";v=\"99\"")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Dnt", "1")
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.192 Safari/537.36")
	req.Header.Set("Ocp-Apim-Subscription-Key", s.key)
	req.Header.Set("Origin", "https://www.systembolaget.se")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Referer", "https://www.systembolaget.se/")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,sv;q=0.8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sresp StockResponse
	sr, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(sr, &sresp)
	if err != nil {
		return nil, err
	}

	return sresp, nil

}
