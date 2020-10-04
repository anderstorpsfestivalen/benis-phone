package systemet

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/secrets"
)

type NewProduct struct {
	ProductID                string  `json:"ProductId"`
	ProductNumber            string  `json:"ProductNumber"`
	ProductNameBold          string  `json:"ProductNameBold"`
	Category                 string  `json:"Category"`
	ProductNumberShort       string  `json:"ProductNumberShort"`
	ProducerName             string  `json:"ProducerName"`
	SupplierName             string  `json:"SupplierName"`
	IsKosher                 bool    `json:"IsKosher"`
	BottleTextShort          string  `json:"BottleTextShort"`
	Seal                     string  `json:"Seal"`
	RestrictedParcelQuantity int     `json:"RestrictedParcelQuantity"`
	IsOrganic                bool    `json:"IsOrganic"`
	IsEthical                bool    `json:"IsEthical"`
	IsWebLaunch              bool    `json:"IsWebLaunch"`
	SellStartDate            string  `json:"SellStartDate"`
	IsCompletelyOutOfStock   bool    `json:"IsCompletelyOutOfStock"`
	IsTemporaryOutOfStock    bool    `json:"IsTemporaryOutOfStock"`
	AlcoholPercentage        float64 `json:"AlcoholPercentage"`
	Volume                   float64 `json:"Volume"`
	Price                    float64 `json:"Price"`
	Country                  string  `json:"Country"`
	OriginLevel1             string  `json:"OriginLevel1"`
	OriginLevel2             string  `json:"OriginLevel2"`
	Vintage                  int     `json:"Vintage"`
	SubCategory              string  `json:"SubCategory"`
	Type                     string  `json:"Type"`
	Style                    string  `json:"Style"`
	AssortmentText           string  `json:"AssortmentText"`
	BeverageDescriptionShort string  `json:"BeverageDescriptionShort"`
	Usage                    string  `json:"Usage"`
	Taste                    string  `json:"Taste"`
	Assortment               string  `json:"Assortment"`
	RecycleFee               float64 `json:"RecycleFee"`
	IsManufacturingCountry   bool    `json:"IsManufacturingCountry"`
	IsRegionalRestricted     bool    `json:"IsRegionalRestricted"`
	IsNews                   bool    `json:"IsNews"`
}

func RequestNewProduct(pn int) (NewProduct, error) {

	s := NewProduct{}
	rpn, err := QueryProductNumberShort(pn)
	if err != nil {
		return NewProduct{}, err
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api-extern.systembolaget.se/product/v1/product/"+strconv.Itoa(rpn.Artikelid), nil)
	req.Header.Add("Ocp-Apim-Subscription-Key", secrets.Loaded.Systemet)
	res, err := client.Do(req)
	if err != nil {
		return NewProduct{}, err
	}
	defer res.Body.Close()

	if res.StatusCode == 401 {
		return NewProduct{}, fmt.Errorf("Systembolaget credentials are not valid")
	}

	if res.StatusCode == 404 {
		return NewProduct{}, fmt.Errorf("Could not find product")
	}

	if res.StatusCode != 200 {
		return NewProduct{}, fmt.Errorf("Rate limited")
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return NewProduct{}, err
	}

	json.Unmarshal(body, &s)

	return s, nil
}
