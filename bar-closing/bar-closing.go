package main

import (
	"fmt"
	"time"
)

func main() {
	now := time.Now()

	add := 1

	if now.Hour() < 3 {
		add = 0
	}

	closing := time.Date(
		now.Year(), now.Month(), now.Day()+add, 01, 00, 00, 000000000, time.UTC)

	fmt.Printf("%T", closing)

	//init the loc
	loc, _ := time.LoadLocation("Europe/Stockholm")

	newtime := closing.In(loc)

	fmt.Println(newtime)

	diff := now.Sub(newtime)
	fmt.Println(diff)

}
