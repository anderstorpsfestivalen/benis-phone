package main

import (
	"os"
	"os/exec"
	"path"
	"time"
)

func main() {
	c := []string{"-f", "alsa", "-i", "hw:2,0", "-af", "\"pan=mono|c0=c1\"", path.Join("temp", "2016-04_11:21"+".flac")}

	dm := exec.Command("ffmpeg", c...)

	time.Sleep(10 * time.Second)

	dm.Process.Signal(os.Interrupt)
}
