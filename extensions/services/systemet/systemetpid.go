package systemet

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"
)

type SystemetPid struct {
	api *SystemetV2
}

type SystemetPidResponse struct {
	R       Product
	Percent string
}

func CreatePidSearch(api *SystemetV2) *SystemetPid {
	return &SystemetPid{
		api: api,
	}
}

func (f *SystemetPid) Get(input string, tmpl string, arguments map[string]string) (string, error) {

	res, err := f.api.SearchForItem(input)

	fmt.Println(err)

	if err != nil {
		// Den här triggar om produkten inte hittas i systembolagets API
		if err.Error() == "No products found" {
			return "Produkten kunde ej hittas", nil
		} else {
			//Alla andra fel triggar riktigt error
			return "", err
		}
	}

	if len(res.Products) > 0 {
		return "", fmt.Errorf("systemet response empty? wtf")
	}

	r := SystemetPidResponse{
		R:       res.Products[0],
		Percent: strings.Replace(strconv.FormatFloat(res.Products[0].AlcoholPercentage, 'f', 1, 64), ".", ",", -1),
	}

	ttmpl, err := template.New("systemetpid").Parse(tmpl)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	err = ttmpl.Execute(buf, r)
	if err != nil {
		return "", err
	}

	return buf.String(), nil

}

func (f *SystemetPid) MaxInputLength() int {
	return 4
}
