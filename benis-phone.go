package main

import (
	"fmt"
	"os"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

var handset_picked_up bool = false

var (
	// Use pin 6
	pin = rpio.Pin(6)
)

func main() {
	fmt.Printf("%T\n", rpio.Pin(6))
	init_gpio()
	for x := 0; x < 100; x++ {
		res := read_gpio()
		fmt.Println(res)
		time.Sleep(1 * time.Second)
	}
}

func init_gpio() {
	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func read_gpio() bool {
	pin.Input()
	res := pin.Read()
	if res == 1 {
		handset_picked_up = true
	}
	if res == 0 {
		handset_picked_up = false
	}
	//defer rpio.Close()
	return handset_picked_up
}
