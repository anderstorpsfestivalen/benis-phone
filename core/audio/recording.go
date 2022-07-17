package audio

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"runtime"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

type Recorder struct {
	isRecording bool
	recordpath  string
	instance    *exec.Cmd
	logger      *logrus.Logger
}

func NewRecorder(recordpath string, logger *logrus.Logger) Recorder {
	return Recorder{
		recordpath: recordpath,
		logger:     logger,
	}
}

func (f *Recorder) Record(subfolder string) {

	go func() {

		if f.isRecording {
			f.Stop()
		}

		os.MkdirAll(f.recordpath, os.ModePerm)
		os.MkdirAll(path.Join(f.recordpath, subfolder), os.ModePerm)

		tm := time.Now()
		recTime := tm.Format("2006-01-02_15-04-05")

		c := []string{"-y", "-f", "alsa", "-i", "hw:0,0", "-af", "pan=mono|c0=c0", path.Join(f.recordpath, subfolder, recTime+".flac")}
		if runtime.GOOS == "darwin" {
			c = []string{"-y", "-f", "avfoundation", "-i", ":2", "-af", "pan=mono|c0=c0", path.Join(f.recordpath, subfolder, recTime+".flac")}
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
	if f.instance != nil {
		f.instance.Process.Signal(os.Interrupt)
	}
	f.isRecording = false
}
