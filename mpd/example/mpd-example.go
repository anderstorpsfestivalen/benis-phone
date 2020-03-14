package main

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/mpd"
)

func main() {
	host := "[::1]:6600"
	m := mpd.Init(host)
	fmt.Println(m)

	m.Add("output.mp3")
	m.Next()
	//m.PlaylistClear()
}
