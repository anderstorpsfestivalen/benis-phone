package idiom

import (
	"io/ioutil"
	"math/rand"
	"reflect"
	"strings"
	"time"
)

// Args is empty: this service takes no arguments.
type Args struct{}

// TemplateData is empty: the service returns a raw line from files/idiom.txt
// and does not render through a template.
type TemplateData struct{}

type Idiom struct{}

func (i *Idiom) ArgsType() reflect.Type     { return reflect.TypeOf(Args{}) }
func (i *Idiom) TemplateType() reflect.Type { return reflect.TypeOf(TemplateData{}) }

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
