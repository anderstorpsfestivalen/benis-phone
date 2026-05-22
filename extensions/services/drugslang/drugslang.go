package drugslang

import (
	"bytes"
	"encoding/csv"
	"math/rand"
	"reflect"
	"strings"
	"text/template"
	"time"
)

// Args is empty: this service takes no arguments.
type Args struct{}

// TemplateData is the value passed into the caller's text/template.
type TemplateData struct {
	Drug  string `desc:"The drug's primary name"`
	Slang string `desc:"Comma-separated list of slang terms"`
}

type Drugslang struct{}

func (d *Drugslang) ArgsType() reflect.Type     { return reflect.TypeOf(Args{}) }
func (d *Drugslang) TemplateType() reflect.Type { return reflect.TypeOf(TemplateData{}) }

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

	ds := TemplateData{
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
