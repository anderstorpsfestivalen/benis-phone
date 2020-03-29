package phone

import "github.com/stianeikeland/go-rpio/v4"

type Phone struct {
	HookPin     rpio.Pin
	HookChannel chan bool
	KeyChannel  chan string
	hookState   bool
}

type FlowPhone interface {
	Init(pin uint8)
	Close()
	State() bool
	GetKeyChannel() chan string
	GetHookChannel() chan bool
}
