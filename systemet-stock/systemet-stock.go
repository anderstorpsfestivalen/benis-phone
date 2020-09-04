package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

type RequestStock struct {
	ProductID string   `json:"productId"`
	SiteIds   []string `json:"siteIds"`
}

type RequestProductInfo struct {
	ProductID string `json:"productId"`
}

type ResponseStock []struct {
	SiteID            string `json:"SiteId"`
	StockTextShort    string `json:"StockTextShort"`
	StockTextLong     string `json:"StockTextLong"`
	ShowStock         bool   `json:"ShowStock"`
	SectionLabel      string `json:"SectionLabel"`
	ShelfLabel        string `json:"ShelfLabel"`
	Shelf             string `json:"Shelf"`
	Section           string `json:"Section"`
	NotYetSaleStarted string `json:"NotYetSaleStarted"`
	IsAgent           bool   `json:"IsAgent"`
	TranslateService  bool   `json:"TranslateService"`
}

type ResponseProductAnalysis struct {
	Products []struct {
		ProductID                       string        `json:"ProductId"`
		ProductNumberShort              string        `json:"ProductNumberShort"`
		Assortment                      string        `json:"Assortment"`
		CustomerOrderSupplySource       string        `json:"CustomerOrderSupplySource"`
		SupplyCode                      interface{}   `json:"SupplyCode"`
		IsNewVintage                    bool          `json:"IsNewVintage"`
		OriginLevel1                    interface{}   `json:"OriginLevel1"`
		OriginLevel2                    interface{}   `json:"OriginLevel2"`
		OriginLevel3                    interface{}   `json:"OriginLevel3"`
		OriginLevel4                    interface{}   `json:"OriginLevel4"`
		OriginLevel5                    interface{}   `json:"OriginLevel5"`
		BrandOrigin                     interface{}   `json:"BrandOrigin"`
		BottleCode                      string        `json:"BottleCode"`
		BottleTypeGroup                 string        `json:"BottleTypeGroup"`
		IsWebLaunch                     bool          `json:"IsWebLaunch"`
		Seal                            interface{}   `json:"Seal"`
		VatCode                         int           `json:"VatCode"`
		PriceExclVat                    float64       `json:"PriceExclVat"`
		PriceInclVatExclRecycleFee      float64       `json:"PriceInclVatExclRecycleFee"`
		PriorPrice                      float64       `json:"PriorPrice"`
		ComparisonPrice                 float64       `json:"ComparisonPrice"`
		SellStartDate                   string        `json:"SellStartDate"`
		SellStartTime                   string        `json:"SellStartTime"`
		BottleTypes                     interface{}   `json:"BottleTypes"`
		IsSellStartDateHighlighted      bool          `json:"IsSellStartDateHighlighted"`
		SellStartSearchURL              string        `json:"SellStartSearchUrl"`
		ProducerName                    string        `json:"ProducerName"`
		ProducerDescription             string        `json:"ProducerDescription"`
		TasteAndUsage                   string        `json:"TasteAndUsage"`
		Production                      string        `json:"Production"`
		CultivationArea                 interface{}   `json:"CultivationArea"`
		Harvest                         interface{}   `json:"Harvest"`
		Soil                            interface{}   `json:"Soil"`
		SupplierName                    string        `json:"SupplierName"`
		IsManufacturingCountry          bool          `json:"IsManufacturingCountry"`
		IsSupplierTemporaryNotAvailable bool          `json:"IsSupplierTemporaryNotAvailable"`
		IsSupplierNotAvailable          bool          `json:"IsSupplierNotAvailable"`
		BackInStockAtSupplier           interface{}   `json:"BackInStockAtSupplier"`
		IsDiscontinued                  bool          `json:"IsDiscontinued"`
		IsCompletelyOutOfStock          bool          `json:"IsCompletelyOutOfStock"`
		IsTemporaryOutOfStock           bool          `json:"IsTemporaryOutOfStock"`
		RestrictedParcelQuantity        int           `json:"RestrictedParcelQuantity"`
		IsRegionalRestricted            bool          `json:"IsRegionalRestricted"`
		IsNewInAssortment               bool          `json:"IsNewInAssortment"`
		IsLimitedEdition                bool          `json:"IsLimitedEdition"`
		IsFsAssortment                  bool          `json:"IsFsAssortment"`
		IsTseAssortment                 bool          `json:"IsTseAssortment"`
		IsTsLsAssortment                bool          `json:"IsTsLsAssortment"`
		IsHidden                        bool          `json:"IsHidden"`
		IsSearchable                    bool          `json:"IsSearchable"`
		IsInAnyStoreSearchAssortment    bool          `json:"IsInAnyStoreSearchAssortment"`
		IsStoreOrderApplicable          bool          `json:"IsStoreOrderApplicable"`
		IsHomeOrderApplicable           bool          `json:"IsHomeOrderApplicable"`
		IsAgentOrderApplicable          bool          `json:"IsAgentOrderApplicable"`
		SeasonName                      interface{}   `json:"SeasonName"`
		IsDishMatchable                 bool          `json:"IsDishMatchable"`
		IsDKI                           bool          `json:"IsDKI"`
		AllergenStatement               string        `json:"AllergenStatement"`
		IngredientStatement             interface{}   `json:"IngredientStatement"`
		AlcoholPercentage               float64       `json:"AlcoholPercentage"`
		TasteSymbols                    string        `json:"TasteSymbols"`
		TasteClockGroup                 string        `json:"TasteClockGroup"`
		TasteClockBitter                int           `json:"TasteClockBitter"`
		TasteClockFruitacid             interface{}   `json:"TasteClockFruitacid"`
		TasteClockBody                  int           `json:"TasteClockBody"`
		TasteClockRoughness             interface{}   `json:"TasteClockRoughness"`
		TasteClockSweetness             int           `json:"TasteClockSweetness"`
		TasteClockCasque                int           `json:"TasteClockCasque"`
		TasteClockSmokiness             interface{}   `json:"TasteClockSmokiness"`
		IsCategoryBeer                  bool          `json:"IsCategoryBeer"`
		IsCategoryBeerOrWhiskey         bool          `json:"IsCategoryBeerOrWhiskey"`
		IsNewsIconVisible               bool          `json:"IsNewsIconVisible"`
		Grapes                          interface{}   `json:"Grapes"`
		RawMaterial                     string        `json:"RawMaterial"`
		SugarContent                    interface{}   `json:"SugarContent"`
		Additives                       interface{}   `json:"Additives"`
		Storage                         interface{}   `json:"Storage"`
		Preservable                     string        `json:"Preservable"`
		HasInbounddeliveries            bool          `json:"HasInbounddeliveries"`
		IsGlutenFree                    bool          `json:"IsGlutenFree"`
		IsEthical                       bool          `json:"IsEthical"`
		EthicalLabel                    interface{}   `json:"EthicalLabel"`
		IsKosher                        bool          `json:"IsKosher"`
		Created                         string        `json:"Created"`
		Modified                        string        `json:"Modified"`
		ShowAdditionalBottleTypes       bool          `json:"ShowAdditionalBottleTypes"`
		OriginLevels                    []interface{} `json:"OriginLevels"`
		TasteSymbolsList                []string      `json:"TasteSymbolsList"`
		IsTasteAndUsageAlone            bool          `json:"IsTasteAndUsageAlone"`
		ImageItem                       []struct {
			ImageURL          string `json:"ImageUrl"`
			ImageAltAttribute string `json:"ImageAltAttribute"`
		} `json:"ImageItem"`
		WebLaunch                   interface{}   `json:"WebLaunch"`
		ProductNutritionHeaders     []interface{} `json:"ProductNutritionHeaders"`
		HasProductImage             bool          `json:"HasProductImage"`
		HasAnyTaste                 bool          `json:"HasAnyTaste"`
		HasSymbolsOrRecycleFee      bool          `json:"HasSymbolsOrRecycleFee"`
		HasTasteAndRestrictions     bool          `json:"HasTasteAndRestrictions"`
		HasRestrictions             bool          `json:"HasRestrictions"`
		HasSymbols                  bool          `json:"HasSymbols"`
		HasAnyTasteClocks           bool          `json:"HasAnyTasteClocks"`
		HasAnyTasteSymbols          bool          `json:"HasAnyTasteSymbols"`
		ProductNumber               string        `json:"ProductNumber"`
		ProductNameBold             string        `json:"ProductNameBold"`
		ProductNameThin             interface{}   `json:"ProductNameThin"`
		PriceInclVat                float64       `json:"PriceInclVat"`
		IsOrganic                   bool          `json:"IsOrganic"`
		IsLightWeightBottle         bool          `json:"IsLightWeightBottle"`
		Volume                      float64       `json:"Volume"`
		Vintage                     int           `json:"Vintage"`
		Country                     string        `json:"Country"`
		Category                    string        `json:"Category"`
		SubCategory                 string        `json:"SubCategory"`
		Type                        string        `json:"Type"`
		Style                       string        `json:"Style"`
		BeverageDescriptionShort    string        `json:"BeverageDescriptionShort"`
		StyleDescription            string        `json:"StyleDescription"`
		RecycleFee                  float64       `json:"RecycleFee"`
		RecycleFeeIndicator         string        `json:"RecycleFeeIndicator"`
		BottleTextShort             string        `json:"BottleTextShort"`
		IsAddableToBasket           bool          `json:"IsAddableToBasket"`
		IsFsTsAssortment            bool          `json:"IsFsTsAssortment"`
		IsBSAssortment              bool          `json:"IsBSAssortment"`
		IsPaAssortment              bool          `json:"IsPaAssortment"`
		ShowAdditionalBsInformation bool          `json:"ShowAdditionalBsInformation"`
		Usage                       string        `json:"Usage"`
		Color                       string        `json:"Color"`
		Aroma                       string        `json:"Aroma"`
		Taste                       string        `json:"Taste"`
		AdditionalInformation       interface{}   `json:"AdditionalInformation"`
	} `json:"Products"`
	StockBalances []interface{} `json:"StockBalances"`
}

func main() {
	// Run before starting the timer
	requestProductAnalytics()
	//requestStockData()

	pollInterval := 5

	tmr := time.Tick(time.Duration(pollInterval) * time.Minute)
	for range tmr {
		requestStockData()
	}
}

func requestStockData() {

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter text: ")
	dtmf_input, _ := reader.ReadString('\n')
	dtmf_input = strings.Replace(dtmf_input, "\n", "", -1)
	fmt.Println(dtmf_input)

	s := ResponseStock{}
	jsonValue, err := json.Marshal(RequestStock{
		//ProductID: "508393",
		//ProdcutID: "507811"
		ProductID: dtmf_input,
		SiteIds:   []string{"0611"},
	})

	if err != nil {
		panic(err)
	}

	res, err := http.Post("https://www.systembolaget.se/api/product/getstockbalance",
		"application/json",
		bytes.NewBuffer(jsonValue))
	if err != nil {
		panic(err)
	}

	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	json.Unmarshal(body, &s)

	for _, site := range s {

		fmt.Println(site.StockTextLong)
	}
}

func requestProductAnalytics() {

	s := ResponseProductAnalysis{}
	jsonValue, err := json.Marshal(RequestProductInfo{
		//ProductID: "508393",
		ProductID: "507811",
		//ProductID: dtmf_input,
		//SiteIds:   []string{"0611"},
		//ProductNumbers: []string{"125512"},
	})

	if err != nil {
		panic(err)
	}

	res, err := http.Post("https://www.systembolaget.se/api/product/GetProductsForAnalytics",
		"application/json",
		bytes.NewBuffer(jsonValue))
	if err != nil {
		panic(err)
	}

	defer res.Body.Close()

	fmt.Println(res)

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	json.Unmarshal(body, &s)

	fmt.Println(s.Products)

}
