package kernelmessage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
)

type KernelMessage struct {
}

type Message struct {
	From    string
	To      string
	Subject string
	Date    string

	Text     string
	Filtered string
}

const url string = "https://mch.anderstorpsfestivalen.se/kernel/messages"

func (k *KernelMessage) Get(input string, tmpl string, arguments map[string]string) (string, error) {

	ttmpl, err := template.New("kernelmessage").Parse(tmpl)
	if err != nil {
		return "", err
	}

	s := []Message{}

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
