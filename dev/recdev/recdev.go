package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"time"
)

func main() {
	c := []string{"-y", "-f", "alsa", "-i", "hw:2,0", "-af", "pan=mono|c0=c0", path.Join("temp", "2016-04_11:21"+".flac")}

	if runtime.GOOS == "darwin" {
		c = []string{"-y", "-f", "avfoundation", "-i", ":1", "-af", `pan=mono|c0=c0`, path.Join("temp", "2016-04_11:21"+".flac")}
	}

	fmt.Println(c)

	dm := exec.Command("ffmpeg", c...)

	stderr, err := dm.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	err = dm.Start()
	if err != nil {
		panic(err)
	}

	time.Sleep(3 * time.Second)

	slurp, _ := ioutil.ReadAll(stderr)
	fmt.Printf("%s\n", slurp)

	dm.Process.Signal(os.Interrupt)
}
