package main

import (
	"fmt"
	"os"

	"github.com/stianeikeland/go-rpio/v4"
)

var handset_picked_up bool = false

var (
	// Use pin 10, corresponds to physical pin 19 on the pi
	pin = rpio.Pin(10)
)

func main() {
	init_gpio()
	res := read_gpio()
	fmt.Println(res)
}

func init_gpio() {
	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func read_gpio() rpio.State {
	pin.Input()
	res := pin.Read()
	defer rpio.Close()
	return res
}
