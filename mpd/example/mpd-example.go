package main

import (
	"gitlab.com/anderstorpsfestivalen/benis-phone/mpd"
)

func main() {
	host := "[::1]:6600"
	m := mpd.Init(host)
}
