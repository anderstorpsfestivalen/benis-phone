package main

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/services/systemet"
)

func main() {

	key, err := systemet.GetKey()
	if err != nil {
		panic(err)
	}

	systemetAPI := systemet.New(key)

	stockresponse, err := systemetAPI.GetStock("508393", "0611")
	if err != nil {
		panic(err)
	}

	fmt.Println(stockresponse[0].Stock)
}
