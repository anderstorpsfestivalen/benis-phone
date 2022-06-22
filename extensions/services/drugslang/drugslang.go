package drugslang

import (
	"bytes"
	"encoding/csv"
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

	// Pick 7 words

	slangs := strings.Split(sl[1], ";")

	var vr []string
	num := 7
	if len(slangs) < num {
		num = len(slangs) - 1
	}

	for i := 1; i < num; i++ {
		rd := rand.Intn(len(slangs))
		vr = append(vr, slangs[rd])
	}

	mr := ""

	for _, w := range vr {
		mr = mr + " " + w + ","
	}

	type DS struct {
		Drug  string
		Slang string
	}

	ds := DS{
		Drug:  sl[0],
		Slang: mr,
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
