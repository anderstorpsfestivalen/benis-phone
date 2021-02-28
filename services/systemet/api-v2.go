package systemet

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
)

type StockResponse []struct {
	ProductID string `json:"productId"`
	StoreID   string `json:"storeId"`
	Shelf     string `json:"shelf"`
	Stock     int    `json:"stock"`
}

type SearchResponse struct {
	Metadata struct {
		DocCount               int `json:"docCount"`
		FullAssortmentDocCount int `json:"fullAssortmentDocCount"`
		NextPage               int `json:"nextPage"`
		PriceRange             struct {
			Min float64 `json:"min"`
			Max float64 `json:"max"`
		} `json:"priceRange"`
		VolumeRange struct {
			Min float64 `json:"min"`
			Max float64 `json:"max"`
		} `json:"volumeRange"`
		AlcoholPercentageRange struct {
			Min float64 `json:"min"`
			Max float64 `json:"max"`
		} `json:"alcoholPercentageRange"`
		SugarContentRange struct {
			Min int `json:"min"`
			Max int `json:"max"`
		} `json:"sugarContentRange"`
		DidYouMeanQuery interface{} `json:"didYouMeanQuery"`
	} `json:"metadata"`
	Products []struct {
		ProductID                string      `json:"productId"`
		ProductNumber            string      `json:"productNumber"`
		ProductNameBold          string      `json:"productNameBold"`
		ProductNameThin          interface{} `json:"productNameThin"`
		Category                 interface{} `json:"category"`
		ProductNumberShort       string      `json:"productNumberShort"`
		ProducerName             string      `json:"producerName"`
		SupplierName             string      `json:"supplierName"`
		IsKosher                 bool        `json:"isKosher"`
		BottleTextShort          string      `json:"bottleTextShort"`
		RestrictedParcelQuantity int         `json:"restrictedParcelQuantity"`
		IsOrganic                bool        `json:"isOrganic"`
		IsEthical                bool        `json:"isEthical"`
		EthicalLabel             interface{} `json:"ethicalLabel"`
		IsWebLaunch              bool        `json:"isWebLaunch"`
		ProductLaunchDate        string      `json:"productLaunchDate"`
		IsCompletelyOutOfStock   bool        `json:"isCompletelyOutOfStock"`
		IsTemporaryOutOfStock    bool        `json:"isTemporaryOutOfStock"`
		AlcoholPercentage        float64     `json:"alcoholPercentage"`
		VolumeText               string      `json:"volumeText"`
		Volume                   float64     `json:"volume"`
		Price                    float64     `json:"price"`
		Country                  string      `json:"country"`
		OriginLevel1             interface{} `json:"originLevel1"`
		OriginLevel2             interface{} `json:"originLevel2"`
		CategoryLevel1           string      `json:"categoryLevel1"`
		CategoryLevel2           string      `json:"categoryLevel2"`
		CategoryLevel3           string      `json:"categoryLevel3"`
		CategoryLevel4           interface{} `json:"categoryLevel4"`
		CustomCategoryTitle      string      `json:"customCategoryTitle"`
		AssortmentText           string      `json:"assortmentText"`
		Usage                    string      `json:"usage"`
		Taste                    string      `json:"taste"`
		TasteSymbols             []string    `json:"tasteSymbols"`
		TasteClockGroupBitter    interface{} `json:"tasteClockGroupBitter"`
		TasteClockGroupSmokiness interface{} `json:"tasteClockGroupSmokiness"`
		TasteClockBitter         int         `json:"tasteClockBitter"`
		TasteClockFruitacid      int         `json:"tasteClockFruitacid"`
		TasteClockBody           int         `json:"tasteClockBody"`
		TasteClockRoughness      int         `json:"tasteClockRoughness"`
		TasteClockSweetness      int         `json:"tasteClockSweetness"`
		TasteClockSmokiness      int         `json:"tasteClockSmokiness"`
		TasteClockCasque         int         `json:"tasteClockCasque"`
		Assortment               string      `json:"assortment"`
		RecycleFee               float64     `json:"recycleFee"`
		IsManufacturingCountry   bool        `json:"isManufacturingCountry"`
		IsRegionalRestricted     bool        `json:"isRegionalRestricted"`
		Packaging                string      `json:"packaging"`
		IsNews                   bool        `json:"isNews"`
		Images                   []struct {
			ImageURL string      `json:"imageUrl"`
			FileType interface{} `json:"fileType"`
			Size     interface{} `json:"size"`
		} `json:"images"`
		IsDiscontinued                  bool          `json:"isDiscontinued"`
		IsSupplierTemporaryNotAvailable bool          `json:"isSupplierTemporaryNotAvailable"`
		SugarContent                    int           `json:"sugarContent"`
		Seal                            []interface{} `json:"seal"`
		Vintage                         interface{}   `json:"vintage"`
		Grapes                          []interface{} `json:"grapes"`
		OtherSelections                 interface{}   `json:"otherSelections"`
		TasteClocks                     []struct {
			Key   string `json:"key"`
			Value int    `json:"value"`
		} `json:"tasteClocks"`
		Color string `json:"color"`
	} `json:"products"`
	Filters []struct {
		Name             string `json:"name"`
		Type             string `json:"type"`
		DisplayName      string `json:"displayName"`
		Description      string `json:"description"`
		IsMultipleChoice bool   `json:"isMultipleChoice"`
		IsActive         bool   `json:"isActive"`
		SearchModifiers  []struct {
			Value    string `json:"value"`
			Count    int    `json:"count"`
			IsActive bool   `json:"isActive"`
		} `json:"searchModifiers"`
		Child interface{} `json:"child"`
	} `json:"filters"`
	FilterMenuItems []interface{} `json:"filterMenuItems"`
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

func (s *SystemetV2) SearchForItem(artikelnr string) (SearchResponse, error) {
	req, err := http.NewRequest("GET", "https://api-extern.systembolaget.se/sb-api-ecommerce/v1/productsearch/search?size=30&page=1&textQuery="+artikelnr+"&isEcoFriendlyPackage=false&isInDepotStockForFastDelivery=false", nil)
	if err != nil {
		return SearchResponse{}, err
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
		return SearchResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	var sresp SearchResponse

	err = json.Unmarshal(body, &sresp)

	if err != nil {
		return SearchResponse{}, err
	}

	return sresp, nil
}
