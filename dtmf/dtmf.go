package dtmf

import (
	"bufio"
	"os"
	"time"
)

type Dtmf struct {
	HookKey chan string
}

func Init() Dtmf {

	d := Dtmf{
		HookKey: make(chan string),
	}

	go d.startRead()

	return d

}

func (d *Dtmf) startRead() {

	reader := bufio.NewReader(os.Stdin)
	ticker := time.NewTicker(100 * time.Millisecond)

	for {
		_ = <-ticker.C

		stdin_read, _ := reader.ReadString('\n')
		first_char := string([]byte(stdin_read)[0])
		d.HookKey <- first_char
	}

}
