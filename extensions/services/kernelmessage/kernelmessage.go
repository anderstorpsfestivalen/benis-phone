package kernelmessage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"reflect"
)

// Args is empty: this service takes no arguments.
type Args struct{}

type KernelMessage struct {
}

func (k *KernelMessage) ArgsType() reflect.Type     { return reflect.TypeOf(Args{}) }
func (k *KernelMessage) TemplateType() reflect.Type { return reflect.TypeOf(TemplateData{}) }

// TemplateData is the value passed into the caller's text/template — a single
// kernel message fetched from the MCH messages endpoint.
type TemplateData struct {
	From    string `desc:"Sender of the message"`
	To      string `desc:"Recipient of the message"`
	Subject string `desc:"Subject line"`
	Date    string `desc:"Timestamp the message was posted"`

	Text     string `desc:"Raw body of the message"`
	Filtered string `desc:"Body with profanity / unspeakable bits filtered out"`
}

const url string = "https://mch.anderstorpsfestivalen.se/kernel/messages"

func (k *KernelMessage) Get(input string, tmpl string, arguments map[string]string) (string, error) {

	ttmpl, err := template.New("kernelmessage").Parse(tmpl)
	if err != nil {
		return "", err
	}

	s := []TemplateData{}

	res, err := http.Get(url)

	if err != nil {
		return "", fmt.Errorf("Could not craft request from API")
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	json.Unmarshal(body, &s)

	buf := new(bytes.Buffer)
	err = ttmpl.Execute(buf, s[0])
	if err != nil {
		return "", err
	}

	return buf.String(), nil

}

func (k *KernelMessage) MaxInputLength() int {
	return 0
}
