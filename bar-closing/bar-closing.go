package main

import (
	"fmt"
	"time"
)

func main() {
	//now := time.Now()

	closing := time.Date(
		2020, 04, 06, 01, 00, 00, 000000000, time.UTC)

	fmt.Printf("%T", closing)

	//init the loc
	loc, _ := time.LoadLocation("Europe/Stockholm")

	//set timezone,
	now := time.Now().In(loc)

	fmt.Println(now)

	diff := now.Sub(closing)
	fmt.Println(diff)

}
