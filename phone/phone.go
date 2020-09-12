package phone

import (
	"bufio"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/stianeikeland/go-rpio"
)

type Phone struct {
	KeyChannel  chan string
	HookChannel chan bool
	Q1          uint8
	Q2          uint8
	Q3          uint8
	Q4          uint8
	StQ         uint8
	Hook    uint8
	pinQ1	rpio.Pin
	pinQ2	rpio.Pin
	pinQ3	rpio.Pin
	pinQ4	rpio.Pin
	pinHook 	rpio.Pin
	pinStQ	rpio.Pin
}

func New(pin1 uint8, pin2 uint8, pin3 uint8, pin4 uint8, pin5 uint8, pin6 uint8) *Phone {
	return &Phone{
		KeyChannel:  make(chan string, 1),
		HookChannel: make(chan bool, 1),
		Q1:          pin1,
		Q2:          pin2,
		Q3:          pin3,
		Q4:          pin4,
		StQ:         pin5,
		Hook:    pin6,
	}
}

func (d *Phone) Init() {

	d.pinQ1 := rpio.Pin(d.Q1)
	d.pinQ2 := rpio.Pin(d.Q2)
	d.pinQ3 := rpio.Pin(d.Q3)
	d.pinQ4 := rpio.Pin(d.Q4)
	d.pinHook := rpio.Pin(d.Hook)
	d.pinStQ := rpio.Pin(d.StQ)
	d.pinQ1.Input()
	d.pinQ2.Input()
	d.pinQ3.Input()
	d.pinQ4.Input()
	d.pinHook.Input()
	d.pinStQ.Input()
	go d.startRead()

}

func (d *Phone) startRead() {

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := []byte(scanner.Text())
		if len(text) > 0 {
			s := string(text[0])
			log.Debug("Keyboard input: " + s)
			if s == "o" || s == "k" {
				var dem bool = false
				if s == "o" {
					dem = true
				}
				select {
				case d.HookChannel <- dem:
					log.Debug("Wrote to hookchannel")
				default:
				}
			} else {
				select {
				case d.KeyChannel <- s:
					log.Debug("Wrote key: " + s + " to keychannel")
				default:
				}
			}

		}

	}

}

func (d *Phone) Close() {

}

func (d *Phone) State() bool {
	return true
}

func (d *Phone) GetKeyChannel() chan string {
	return d.KeyChannel
}

func (d *Phone) GetHookChannel() chan bool {
	return d.HookChannel
}
