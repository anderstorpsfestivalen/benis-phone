package main

import (
	"fmt"
	"strconv"

	"gitlab.com/anderstorpsfestivalen/benis-phone/services/systemet"
)

func main() {

	key, err := systemet.GetKey()
	if err != nil {
		panic(err)
	}

	systemetAPI := systemet.New(key)

	err = systemet.Init()
	art, err := systemet.QueryProductNumberShort(11392)

	stockresponse, err := systemetAPI.GetStock(strconv.Itoa(art.Artikelid), "0611")
	if err != nil {
		panic(err)
	}

	fmt.Println(stockresponse[0].Stock)

}
