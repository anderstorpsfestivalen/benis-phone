package virtual

import (
	"bufio"
	"fmt"
	"os"
)

type Virtual struct {
	KeyChannel  chan string
	HookChannel chan bool
}

func New() *Virtual {
	return &Virtual{
		KeyChannel:  make(chan string, 1),
		HookChannel: make(chan bool, 1),
	}
}

func (d *Virtual) Init(p uint8) {

	d.startRead()

}

func (d *Virtual) startRead() {

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := []byte(scanner.Text())
		if len(text) > 0 {
			s := string(text[0])
			fmt.Println("Keyboard Input: " + s)
			if s == "o" || s == "k" {
				var dem bool = false
				if s == "o" {
					dem = true
				}
				select {
				case d.HookChannel <- dem:
					fmt.Println("Wrote Hookchannel")
				default:
				}
			} else {
				select {
				case d.KeyChannel <- s:
					fmt.Println("Wrote Keychannel")
				default:
				}
			}

		}

	}

}

func (d *Virtual) Close() {

}

func (d *Virtual) State() bool {
	return true
}

func (d *Virtual) GetKeyChannel() chan string {
	return d.KeyChannel
}

func (d *Virtual) GetHookChannel() chan bool {
	return d.HookChannel
}
