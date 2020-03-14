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
	c.PlaylistClear()
	conn, err := mpd.Dial("tcp", c.Host)
	if err != nil {
		fmt.Println("BENIS")
	}
	c.m = conn
	return c
}

func (c MpdClient) Close() {
	c.m.Close()
}

func (c MpdClient) Add(f string) {

	c.m.Update(f)

	fmt.Println("Adding %s\n", f)
	c.m.Add(f)
}

func (c MpdClient) Next() {
	fmt.Println("Playging next in playlist\n")
	c.m.Next()
}

func (c MpdClient) PlaylistClear() {

	fmt.Println("Clearing playlist\n")

	//err = conn.PlaylistClear("default")
	err := c.m.Clear()
	if err != nil {
		log.Fatalln(err)
	}
}
