// package main

// import (
// 	"fmt"

// 	"gitlab.com/anderstorpsfestivalen/benis-phone/services/systemet"
// )

// func main() {

// 	key, err := systemet.GetKey()
// 	if err != nil {
// 		panic(err)
// 	}

// 	systemetAPI := systemet.New(key)

// 	item, err := systemetAPI.SearchForItem("11393")
// 	if err != nil {
// 		panic(err)
// 	}

// 	fmt.Println(item.Products[0].Usage)

// 	// err = systemet.Init()
// 	// art, err := systemet.QueryProductNumberShort(11392)

// 	// stockresponse, err := systemetAPI.GetStock(strconv.Itoa(art.Artikelid), "0611")
// 	// if err != nil {
// 	// 	panic(err)
// 	// }

// 	// fmt.Println(stockresponse[0].Stock)

// }
