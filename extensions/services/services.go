package services

import (
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/drogslang"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/drugslang"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/flacornot"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/idiom"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/kernelmessage"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/train"
)

type Service interface {
	Get(input string, template string, arguments map[string]string) (string, error)
	MaxInputLength() int
}

type ServiceResponse struct {
	Message string
	State   map[string]string
}

var ServiceRegistry = map[string]Service{
	"flacornot": &flacornot.FlacOrNot{},
	// "barclosing": &barclosing.BarClosing{},
	"idiom":         &idiom.Idiom{},
	"traintimes":    &train.Train{},
	"drogslang":     &drogslang.Drogslang{},
	"drugslang":     &drugslang.Drugslang{},
	"kernelmessage": &kernelmessage.KernelMessage{},
}

func AddService(name string, s Service) {
	ServiceRegistry[name] = s
}
