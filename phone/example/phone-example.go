package main

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/phone"
)

func main() {

	ph := phone.Init(6)
	state := ph.State()
	fmt.Println(state)

}
