package main

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/systemet"
)

func main() {
	fmt.Println("Hej")
	fmt.Println(systemet.RequestStockData("508393"))
}
