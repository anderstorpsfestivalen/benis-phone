package audio

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"runtime"
	"syscall"

	"github.com/sirupsen/logrus"
)

type Recorder struct {
	isRecording bool
	recordpath  string
	instance    *exec.Cmd
	logger      *logrus.Logger
	alsaDevice  string
}

func NewRecorder(alsaDevice string, recordpath string, logger *logrus.Logger) Recorder {
	return Recorder{
		recordpath: recordpath,
		logger:     logger,
		alsaDevice: alsaDevice,
	}
}

func (f *Recorder) Record(filename string) {

	go func() {

		if f.isRecording {
			f.Stop()
		}

		c := []string{"-y", "-f", "alsa", "-i", "hw:0,0", "-af", "pan=mono|c0=c0", path.Join(f.recordpath, filename+".flac")}

		if runtime.GOOS == "darwin" {
			c = []string{"-y", "-f", "avfoundation", "-i", ":0", "-af", "pan=mono|c0=c0", path.Join(f.recordpath, filename+".flac")}
		}

		f.instance = exec.Command("ffmpeg", c...)
		stderr, err := f.instance.StderrPipe()
		if err != nil {
			f.isRecording = false
			f.logger.Error(err)
		}

		err = f.instance.Start()
		if err != nil {
			f.isRecording = false
			f.logger.Error(err)
		}

		slurp, err := ioutil.ReadAll(stderr)
		if err != nil {
			f.isRecording = false
			f.logger.Error(err)
		}

		if err := f.instance.Wait(); err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {

				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					ex := status.ExitStatus()
					if ex == -1 || ex == 255 {
					} else {
						f.logger.Error("FFMPEG DID NOT EXIT CLEANLY")
						f.logger.Errorf("%s\n", slurp)
					}
				}
			}
		}

		f.isRecording = true
	}()
}

func (f *Recorder) Stop() {

	f.instance.Process.Signal(os.Interrupt)
	f.isRecording = false
}
