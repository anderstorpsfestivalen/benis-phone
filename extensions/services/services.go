package services

import (
	"reflect"

	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/drogslang"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/drugslang"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/flacornot"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/idiom"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/kernelmessage"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/saunatemp"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/train"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/weather"
)

// Service is the contract every IVR extension must satisfy. ArgsType and
// TemplateType return the Go types describing the service's TOML args block
// and the value passed into its text/template — the editor UI reflects on
// both via tools/typegen to render typed forms instead of free-text input.
type Service interface {
	Get(input string, template string, arguments map[string]string) (string, error)
	MaxInputLength() int
	ArgsType() reflect.Type
	TemplateType() reflect.Type
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
	"weather":       &weather.Weather{},
	"saunatemp":     &saunatemp.Saunatemp{},
}

func AddService(name string, s Service) {
	ServiceRegistry[name] = s
}
