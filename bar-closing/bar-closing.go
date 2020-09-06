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

	//init the loc
	loc, _ := time.LoadLocation("Europe/Stockholm")

	newtime := closing.In(loc)

	fmt.Printf("closing: %v\n", newtime)

	diff := now.Sub(newtime).Round(time.Minute)
	fmt.Printf("diff is type: %T\n", diff)
	fmt.Printf("diff is %v \n", diff)

	fmt.Printf("%v\n", diff.Hours)
	fmt.Printf("%v\n", diff.Minutes)
}
