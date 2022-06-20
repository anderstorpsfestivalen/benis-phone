package virtual

import (
	"os"

	"github.com/eiannone/keyboard"
	log "github.com/sirupsen/logrus"
)

type Virtual struct {
	KeyChannel  chan string
	HookChannel chan bool

	hookState bool
}

func New() *Virtual {
	return &Virtual{
		KeyChannel:  make(chan string, 1),
		HookChannel: make(chan bool, 1),
	}
}

func (d *Virtual) Init() error {
	keysEvents, err := keyboard.GetKeys(1)
	if err != nil {
		return err
	}

	go d.startRead(keysEvents)

	log.Info(`Virtual phone started
	
	Key Mapping
	
	0-9: 0-9
	U: *
	J: #

	O: lift hook
	K: slam hook

	ESC or Q: quit benis-phone`)

	return nil

}

func (d *Virtual) startRead(k <-chan keyboard.KeyEvent) {
	defer func() {
		_ = keyboard.Close()
	}()

	for {
		event := <-k
		if event.Err != nil {
			log.Fatal(event.Err)
		}

		log.Trace("You pressed: rune %q, key %X\r\n", event.Rune, event.Key)

		// Quit
		if event.Key == keyboard.KeyEsc ||
			event.Rune == 'q' ||
			event.Rune == 'Q' {
			log.Info("Escape pressed - Exiting")
			os.Exit(0)
		}

		s := string(event.Rune)

		if s == "o" || s == "k" {
			var dem bool = false
			if s == "o" {
				dem = true
				d.hookState = true
			}

			if s == "k" {
				d.hookState = false
			}

			select {
			case d.HookChannel <- dem:
				log.Debug("Wrote to hookchannel")
			default:
			}
		} else {
			//hack to convert u to * and j to #
			if s == "u" {
				s = "10"
			}
			if s == "j" {
				s = "11"
			}

			select {
			case d.KeyChannel <- s:
				log.Debug("Wrote key: " + s + " to keychannel")
			default:

			}

		}
	}
}

// Saving this shit code because I know that I needed it at some point?
// Can't remember though lol
// maybe there was some linux lib issue with this approach?
//
// func (d *Virtual) startRead() {

// 	scanner := bufio.NewScanner(os.Stdin)
// 	for scanner.Scan() {
// 		text := []byte(scanner.Text())
// 		if len(text) > 0 {
// 			s := string(text[0])
// 			log.Debug("Keyboard input: " + s)
// 			if s == "o" || s == "k" {
// 				var dem bool = false
// 				if s == "o" {
// 					dem = true
// 					d.hookState = true
// 				}

// 				if s == "k" {
// 					d.hookState = false
// 				}

// 				select {
// 				case d.HookChannel <- dem:
// 					log.Debug("Wrote to hookchannel")
// 				default:
// 				}
// 			} else {
// 				//hack to convert u to * and j to #
// 				if s == "u" {
// 					s = "10"
// 				}
// 				if s == "j" {
// 					s = "11"
// 				}

// 				select {		case d.KeyChannel <- s:
// 					log.Debug("Wrote key: " + s + " to keychannel")
// 				default:
//
// 		}
// 			}

// 		}

// 	}

// }

func (d *Virtual) Close() {

}

func (d *Virtual) State() bool {
	return d.hookState
}

func (d *Virtual) GetKeyChannel() chan string {
	return d.KeyChannel
}

func (d *Virtual) GetHookChannel() chan bool {
	return d.HookChannel
}
