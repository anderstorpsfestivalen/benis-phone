package systemet

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
)

type Artiklar struct {
	XMLName   xml.Name `xml:"artiklar"`
	Text      string   `xml:",chardata"`
	Xsd       string   `xml:"xsd,attr"`
	Xsi       string   `xml:"xsi,attr"`
	SkapadTid string   `xml:"skapad-tid"`
	Info      struct {
		Text       string `xml:",chardata"`
		Meddelande string `xml:"meddelande"`
	} `xml:"info"`
	Artikel []Artikel `xml:"artikel"`
}

type Artikel struct {
	Text               string `xml:",chardata" json:"-"`
	Nr                 int    `xml:"nr" json:"nr"`
	Artikelid          int    `xml:"Artikelid" json:"Artikelid"`
	Varnummer          int    `xml:"Varnummer" json:"Varunummer"`
	Namn               string `xml:"Namn" json:"-"`
	Namn2              string `xml:"Namn2" json:"-"`
	Prisinklmoms       string `xml:"Prisinklmoms" json:"-"`
	Volymiml           string `xml:"Volymiml" json:"-"`
	PrisPerLiter       string `xml:"PrisPerLiter" json:"-"`
	Saljstart          string `xml:"Saljstart" json:"-"`
	Utgått             string `xml:"Utgått" json:"-"`
	Varugrupp          string `xml:"Varugrupp" json:"-"`
	Typ                string `xml:"Typ" json:"-"`
	Stil               string `xml:"Stil" json:"-"`
	Forpackning        string `xml:"Forpackning" json:"-"`
	Forslutning        string `xml:"Forslutning" json:"-"`
	Ursprung           string `xml:"Ursprung" json:"-"`
	Ursprunglandnamn   string `xml:"Ursprunglandnamn" json:"-"`
	Producent          string `xml:"Producent" json:"-"`
	Leverantor         string `xml:"Leverantor" json:"-"`
	Argang             string `xml:"Argang" json:"-"`
	Provadargang       string `xml:"Provadargang" json:"-"`
	Alkoholhalt        string `xml:"Alkoholhalt" json:"-"`
	Sortiment          string `xml:"Sortiment" json:"-"`
	SortimentText      string `xml:"SortimentText" json:"-"`
	Ekologisk          string `xml:"Ekologisk" json:"-"`
	Etiskt             string `xml:"Etiskt" json:"-"`
	Koscher            string `xml:"Koscher" json:"-"`
	RavarorBeskrivning string `xml:"RavarorBeskrivning" json:"-"`
}

var artMap map[int]Artikel

func Init() error {

	artMap = make(map[int]Artikel)

	f, err := os.Open("files/bolaget-index.json")
	if err != nil {
		return err
	}

	dec := json.NewDecoder(f)
	err = dec.Decode(&artMap)
	if err != nil {
		return err
	}

	return nil
}

func QueryProductNumberShort(shortNumber int) (Artikel, error) {

	if val, ok := artMap[shortNumber]; ok {
		return val, nil
	}
	return Artikel{}, fmt.Errorf("Could not find product number short")
}
