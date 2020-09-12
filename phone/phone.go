package phone

import (
	"bufio"
	"os"

	log "github.com/sirupsen/logrus"
)

type Phone struct {
	KeyChannel  chan string
	HookChannel chan bool
	Q1          uint8
	Q2          uint8
	Q3          uint8
	Q4          uint8
	StQ         uint8
	HookGPIO    uint8
}

func New(pin1 uint8, pin2 uint8, pin3 uint8, pin4 uint8, pin5, uint8, pin6 uint8) *Phone {
	return &Phone{
		KeyChannel:  make(chan string, 1),
		HookChannel: make(chan bool, 1),
		Q1:          pin1,
		Q2:          pin2,
		Q3:          pin3,
		Q4:          pin4,
		StQ:         pin5,
		HookGPIO:    pin6,
	}
}

func (d *Phone) Init() {

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
