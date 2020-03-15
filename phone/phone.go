package phone

import (
	"fmt"
	"os"

	"github.com/stianeikeland/go-rpio/v4"
)

type Phone struct {
	GPIO_Pin      *rpio.Pin(6)
	handset_state bool
}

func Init_GPIO() {
	var p *Phone

	_ = rpio.Pin(p.GPIO_Pin)
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
