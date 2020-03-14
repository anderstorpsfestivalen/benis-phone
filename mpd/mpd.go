package mpd

import (
	"fmt"
	"log"

	"github.com/fhs/gompd/mpd"
)

type MpdClient struct {
	Host string
}

func Init(h string) MpdClient {
	var c MpdClient
	c.Host = h
	c.PlaylistClear()
	return c
}

func (c MpdClient) Add(f string) {
	conn, err := mpd.Dial("tcp", c.Host)
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	conn.Update(f)

	fmt.Println("Adding %s\n", f)
	conn.Add(f)
}

func (c MpdClient) Next() {
	conn, err := mpd.Dial("tcp", c.Host)
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	fmt.Println("Playging next in playlist\n")
	conn.Next()
}

func (c MpdClient) PlaylistClear() {
	conn, err := mpd.Dial("tcp", c.Host)
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	fmt.Println("Clearing playlist\n")

	//err = conn.PlaylistClear("default")
	err = conn.Clear()
	if err != nil {
		log.Fatalln(err)
	}
}
