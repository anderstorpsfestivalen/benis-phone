package mpd

import (
	"fmt"
	"log"
	"time"

	"github.com/fhs/gompd/mpd"
)

type MpdClient struct {
	Host string
	m    *mpd.Client
}

func Init(h string) (MpdClient, error) {
	var c MpdClient
	c.Host = h
	conn, err := mpd.Dial("tcp", c.Host)
	if err != nil {
		return MpdClient{}, err
	}
	c.m = conn
	c.m.Clear()
	c.m.Consume(true)
	return c, nil
}

func (c MpdClient) Close() {
	c.m.Close()
}

func (c MpdClient) Add(f string) {
	c.m.Update(f)
	time.Sleep(50 * time.Millisecond)

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

func (c MpdClient) PlayBlocking() {
	fmt.Println("Play first in queue blocking\n")

	err := c.m.Play(-1)
	if err != nil {
		log.Fatalln(err)
	}

	for {
		attr, _ := c.m.Status()
		if state, ok := attr["state"]; ok {
			if state != "play" {
				break
			}
		} else {
			break
		}

		time.Sleep(time.Microsecond * 100)
	}

	fmt.Println("release")

}

func (c MpdClient) State() (string, error) {
	attr, err := c.m.Status()
	if state, ok := attr["state"]; ok {
		return state, nil
	} else {
		return "", err
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
