package main

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/phone"
)

func main() {

	ph := phone.Init(6)

	fmt.Println(ph.State())
	for {
		select {
		case s := <-ph.HookChannel:
			fmt.Println(s)
		default:
			fmt.Println("BEEEEEEEENIS")
		}
	}

}
