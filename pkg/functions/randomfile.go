package functions

import (
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"time"
)

type RandomFile struct {
	Folder string `toml:"folder"`
}

func (f RandomFile) GetPlayable() (Playable, error) {

	files, err := ioutil.ReadDir(f.Folder)
	if err != nil {
		return Playable{}, err
	}

	rand.Seed(time.Now().UnixNano())
	number := rand.Intn(len(files) - 1)
	filename := filepath.Join(filepath.Clean(f.Folder), files[number].Name())

	return Playable{
		File: File{
			Source: filename,
		},
	}, nil
}
