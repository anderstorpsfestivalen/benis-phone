package systemet

import (
	"bytes"
	"strconv"
	"text/template"
)

type SystemetStock struct {
	api *SystemetV2
}

func CreateStock(api *SystemetV2) *SystemetStock {
	return &SystemetStock{
		api: api,
	}
}

func (f *SystemetStock) Get(input string, tmpl string, arguments map[string]string) (string, error) {
	stock, err := f.api.GetStock(arguments["productid"], arguments["storeid"])
	if err != nil {
		return "", err
	}

	ttmpl, err := template.New("systemetstock").Parse(tmpl)
	if err != nil {
		return "", err
	}

	type DS struct {
		Stock string
	}

	ds := DS{
		Stock: strconv.Itoa(stock[0].Stock),
	}

	buf := new(bytes.Buffer)
	err = ttmpl.Execute(buf, ds)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (f *SystemetStock) MaxInputLength() int {
	return 0
}
