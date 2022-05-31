package idiom

import (
	"io/ioutil"
	"math/rand"
	"strings"
	"time"
)

type Idiom struct{}

func (f *Idiom) Get(input string, tmpl string, arguments map[string]string) (string, error) {
	data, err := ioutil.ReadFile("files/idiom.txt")
	if err != nil {
		return "", nil
	}

	lines := strings.Split(string(data), "\n")

	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	randomIndex := r1.Intn(len(lines))

	message := lines[randomIndex]

	return message, nil
}

func (t *Idiom) MaxInputLength() int {
	return 0
}
