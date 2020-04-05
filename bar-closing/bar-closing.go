package main

import (
	"fmt"
	"time"
)

func main() {
	now := time.Now()

	closing := time.Date(
		now.Year(), now.Month(), now.Day()+1, 01, 00, 00, 000000000, time.UTC)

	fmt.Printf("%T", closing)

	//init the loc
	loc, _ := time.LoadLocation("Europe/Stockholm")

	newtime := closing.In(loc)

	fmt.Println(newtime)

	diff := now.Sub(newtime)
	fmt.Println(diff)

}
