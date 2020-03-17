package phone

import (
	"fmt"
	"os"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

type Phone struct {
	HookPin     rpio.Pin
	HookChannel chan bool
	hookState   bool
}

func Init(physicalPin uint8) Phone {

	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	p := Phone{
		HookPin:     rpio.Pin(physicalPin),
		HookChannel: make(chan bool),
	}

	go p.startRead()

	return p

}

func (p *Phone) Close() {
	rpio.Close()
}

func (p *Phone) startRead() {
	p.HookPin.Input()

	ticker := time.NewTicker(100 * time.Millisecond)

	for {
		//BLOCK
		_ = <-ticker.C

		res := p.HookPin.Read()

		var t bool
		if res == 1 {
			t = true
		}
		if res == 0 {
			t = false
		}

		if t != p.hookState {
			p.HookChannel <- t
		}

		p.hookState = t

	}

}

func (p *Phone) State() bool {

	return p.hookState
}
