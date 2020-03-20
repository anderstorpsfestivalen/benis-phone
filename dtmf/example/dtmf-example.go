package main

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/dtmf"
)

func main() {
	dt := dtmf.Init()

	s := <-dt.HookChannel
	fmt.Println(s)

}
