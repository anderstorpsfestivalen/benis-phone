package drugslang

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math/rand"
	"strings"
	"text/template"
	"time"
)

type Drugslang struct{}

func (f *Drugslang) Get(input string, tmpl string, arguments map[string]string) (string, error) {
	ttmpl, err := template.New("drogslang").Parse(tmpl)
	if err != nil {
		return "", err
	}

	csvReader := csv.NewReader(strings.NewReader(data))
	d, err := csvReader.ReadAll()
	if err != nil {
		return "", err
	}

	rand.Seed(time.Now().UnixNano())

	sl := d[rand.Intn(len(d)-1)]

	fmt.Println(sl)

	//var random []int
	//for i, _

	type DS struct {
		Slang string
	}

	ds := DS{
		Slang: "brexit",
	}

	buf := new(bytes.Buffer)
	err = ttmpl.Execute(buf, ds)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (t *Drugslang) MaxInputLength() int {
	return 0
}
