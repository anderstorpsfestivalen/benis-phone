package mpd

import (
	"log"

	"github.com/fhs/gompd/mpd"
)

type MpdClient struct {
	Host string
}

var c MpdClient

func Init(h string) {
	c.Host = h
	c.playlistClear()
}

func (c MpdClient) Add(f string) {
	conn, err := mpd.Dial("tcp", c.Host)
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	log.Printf("Adding %s\n", f)

	conn.Add(f)
}

func (c MpdClient) Next() {
	conn, err := mpd.Dial("tcp", c.Host)
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	log.Printf("Playging next in playlist\n")
	conn.Next()
}

func (c MpdClient) playlistClear() {
	conn, err := mpd.Dial("tcp", c.Host)
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	log.Printf("Clearing playlist\n")

	err = conn.PlaylistClear("test")
	if err != nil {
		log.Fatalln(err)
	}
}
