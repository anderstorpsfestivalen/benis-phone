package systemet

import (
	"bytes"
	"reflect"
	"strconv"
	"text/template"
)

// SystemetStockArgs is the typed view of the TOML `args` map for this service.
type SystemetStockArgs struct {
	ProductID string `schema:"required" desc:"Systembolaget product ID"`
	StoreID   string `schema:"required" desc:"Store ID to query stock at"`
}

// SystemetStockTemplate is the value passed into the caller's text/template.
type SystemetStockTemplate struct {
	Stock string `desc:"Number of bottles in stock, as a string"`
}

type SystemetStock struct {
	api *SystemetV2
}

func (f *SystemetStock) ArgsType() reflect.Type     { return reflect.TypeOf(SystemetStockArgs{}) }
func (f *SystemetStock) TemplateType() reflect.Type { return reflect.TypeOf(SystemetStockTemplate{}) }

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
