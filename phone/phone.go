package phone

import (
	"fmt"
	"os"

	"github.com/stianeikeland/go-rpio/v4"
)

type Phone struct {
	GPIO_Pin      rpio.Pin
	handset_state bool
}

func Init(physicalPin uint8) Phone {

	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return Phone{
		GPIO_Pin: rpio.Pin(physicalPin),
	}

}

func (p *Phone) Close() {
	rpio.Close()
}

func (p *Phone) State() bool {
	p.GPIO_Pin.Input()
	res := p.GPIO_Pin.Read()
	if res == 1 {
		p.handset_state = true
	}
	if res == 0 {
		p.handset_state = false
	}
	return p.handset_state
}
