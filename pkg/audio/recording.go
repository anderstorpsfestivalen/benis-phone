package audio

import (
	"os"
	"os/exec"
	"path"
	"runtime"
)

type Recorder struct {
	isRecording bool
	recordpath  string
	instance    *exec.Cmd
	alsaDevice  string
}

func NewRecorder(alsaDevice string, recordpath string) Recorder {
	return Recorder{
		recordpath: recordpath,
		alsaDevice: alsaDevice,
	}
}

func (f *Recorder) Record(filename string) error {

	if f.isRecording {
		f.Stop()
	}

	c := []string{"-y", "-f", "alsa", "-i", "hw:2,0", "-af", "'pan=mono|c0=c0'", path.Join(f.recordpath, filename+".flac")}

	if runtime.GOOS == "darwin" {
		c = []string{"-f", "avfoundation", "-i", ":1", path.Join(f.recordpath, filename+".flac")}
	}

	f.instance = exec.Command("ffmpeg", c...)
	err := f.instance.Start()
	if err != nil {
		f.isRecording = false
		return err
	}

	f.isRecording = true
	return nil
}

func (f *Recorder) Stop() {

	f.instance.Process.Signal(os.Interrupt)
}
