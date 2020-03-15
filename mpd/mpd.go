package mpd

import (
	"fmt"
	"log"

	"github.com/fhs/gompd/mpd"
)

type MpdClient struct {
	Host string
	m    *mpd.Client
}

func Init(h string) MpdClient {
	var c MpdClient
	c.Host = h
	conn, err := mpd.Dial("tcp", c.Host)
	if err != nil {
		log.Fatalln(err)
	}
	c.m = conn
	c.Clear()
	c.m.Consume(true)
	return c
}

func (c MpdClient) Close() {
	c.m.Close()
}

func (c MpdClient) Add(f string) {
	c.m.Update(f)

	fmt.Printf("Adding %s\n", f)
	c.m.Add(f)
}

func (c MpdClient) Next() {
	fmt.Println("Playging next in playlist\n")
	c.m.Next()
}

func (c MpdClient) Play() {
	fmt.Println("Play first in queue\n")
	err := c.m.Play(-1)
	if err != nil {
		log.Fatalln(err)
	}
}

func (c MpdClient) Clear() {
	fmt.Println("Clearing playlist\n")

	//err = conn.PlaylistClear("default")
	err := c.m.Clear()
	if err != nil {
		log.Fatalln(err)
	}
}
