package phone

import (
	"fmt"
	"time"

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
	Hook        uint8
	pinQ1       rpio.Pin
	pinQ2       rpio.Pin
	pinQ3       rpio.Pin
	pinQ4       rpio.Pin
	pinHook     rpio.Pin
	pinStQ      rpio.Pin
	inputState  bool

	hookState bool
}

//New creates a new phone, pin order is Q1-Q4, stq for button press and hook for hook state GPIO.
func New(q1 uint8, q2 uint8, q3 uint8, q4 uint8, stq uint8, hook uint8) *Phone {
	return &Phone{
		KeyChannel:  make(chan string, 1),
		HookChannel: make(chan bool, 1),
		Q1:          q1,
		Q2:          q2,
		Q3:          q3,
		Q4:          q4,
		StQ:         stq,
		Hook:        hook,
	}
}

func (d *Phone) Init() error {

	err := rpio.Open()
	if err != nil {
		return err
	}

	d.pinQ1 = rpio.Pin(d.Q1)
	d.pinQ2 = rpio.Pin(d.Q2)
	d.pinQ3 = rpio.Pin(d.Q3)
	d.pinQ4 = rpio.Pin(d.Q4)
	d.pinHook = rpio.Pin(d.Hook)
	d.pinStQ = rpio.Pin(d.StQ)

	d.pinQ1.Input()
	d.pinQ2.Input()
	d.pinQ3.Input()
	d.pinQ4.Input()
	d.pinHook.Input()
	d.pinStQ.Input()
	go d.startRead()

	return nil

}

func (d *Phone) startRead() {
	for {
		//Check hook state
		hookRead := d.pinHook.Read()
		hookBool := (hookRead != 0)
		if d.hookState != hookBool {
			time.Sleep(5 * time.Millisecond)
			d.hookState = hookBool
			select {
			case d.HookChannel <- hookBool:
				log.Debug("Wrote to hookchannel")
			default:
			}
		}

		//Check key press
		if d.hookState {
			resStQ := d.pinStQ.Read()

			if resStQ == 1 && !d.inputState {
				d.inputState = true
				time.Sleep(5 * time.Millisecond)
				var bs byte = (0x00 | (byte(d.pinQ1.Read()) << 0) | (byte(d.pinQ2.Read()) << 1) | (byte(d.pinQ3.Read()) << 2) | (byte(d.pinQ4.Read()) << 3))

				var pressed string
				switch bs {
				case 0x01:
					pressed = "1"
				case 0x02:
					pressed = "2"
				case 0x03:
					pressed = "3"
				case 0x04:
					pressed = "4"
				case 0x05:
					pressed = "5"
				case 0x06:
					pressed = "6"
				case 0x07:
					pressed = "7"
				case 0x08:
					pressed = "8"
				case 0x09:
					pressed = "9"
				case 0x0A:
					pressed = "0"
				case 0x0B:
					pressed = "*"
				case 0x0C:
					pressed = "#"
				default:
					pressed = "0"
				}

				select {
				case d.KeyChannel <- pressed:
					fmt.Println("Wrote key: " + pressed + " to keychannel")
				default:
				}
			}

			if resStQ == 0 && d.inputState {
				d.inputState = false
			}

		}

	}
}

func (d *Phone) Close() {

}

func (d *Phone) State() bool {
	return d.hookState
}

func (d *Phone) GetKeyChannel() chan string {
	return d.KeyChannel
}

func (d *Phone) GetHookChannel() chan bool {
	return d.HookChannel
}
