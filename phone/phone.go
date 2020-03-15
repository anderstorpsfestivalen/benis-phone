package phone

import (
	"fmt"
	"os"

	"github.com/stianeikeland/go-rpio/v4"
)

type Phone struct {
	Pin_GPIO      rpio.Pin
	handset_state bool
}

func Init_GPIO() {
	var p *Phone
	p.Pin_GPIO = rpio.Pin(6)
	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func Close() {
	defer rpio.Close()
}

func State() bool {
	Init_GPIO()
	var p *Phone
	p.Pin_GPIO.Input()
	res := p.Pin_GPIO.Read()
	if res == 1 {
		p.handset_state = true
	}
	if res == 0 {
		p.handset_state = false
	}
	return p.handset_state
}
