package systemet

import (
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
	Text               string `xml:",chardata"`
	Nr                 int    `xml:"nr"`
	Artikelid          int    `xml:"Artikelid"`
	Varnummer          int    `xml:"Varnummer"`
	Namn               string `xml:"Namn"`
	Namn2              string `xml:"Namn2"`
	Prisinklmoms       string `xml:"Prisinklmoms"`
	Volymiml           string `xml:"Volymiml"`
	PrisPerLiter       string `xml:"PrisPerLiter"`
	Saljstart          string `xml:"Saljstart"`
	Utgått             string `xml:"Utgått"`
	Varugrupp          string `xml:"Varugrupp"`
	Typ                string `xml:"Typ"`
	Stil               string `xml:"Stil"`
	Forpackning        string `xml:"Forpackning"`
	Forslutning        string `xml:"Forslutning"`
	Ursprung           string `xml:"Ursprung"`
	Ursprunglandnamn   string `xml:"Ursprunglandnamn"`
	Producent          string `xml:"Producent"`
	Leverantor         string `xml:"Leverantor"`
	Argang             string `xml:"Argang"`
	Provadargang       string `xml:"Provadargang"`
	Alkoholhalt        string `xml:"Alkoholhalt"`
	Sortiment          string `xml:"Sortiment"`
	SortimentText      string `xml:"SortimentText"`
	Ekologisk          string `xml:"Ekologisk"`
	Etiskt             string `xml:"Etiskt"`
	Koscher            string `xml:"Koscher"`
	RavarorBeskrivning string `xml:"RavarorBeskrivning"`
}

var artMap map[int]Artikel

func Init() {

	artMap = make(map[int]Artikel)

	atk := Artiklar{}

	xfile, err := os.Open("files/bolaget-index.xml")
	if err != nil {
		panic(err)
	}

	dec := xml.NewDecoder(xfile)

	dec.Decode(&atk)

	for _, v := range atk.Artikel {
		artMap[v.Varnummer] = v
	}

}

func QueryProductNumberShort(shortNumber int) (Artikel, error) {

	if val, ok := artMap[shortNumber]; ok {
		return val, nil
	}
	return Artikel{}, fmt.Errorf("Could not find product number short")
}
