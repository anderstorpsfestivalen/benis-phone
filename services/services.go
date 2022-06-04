package services

import (
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/drogslang"
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/flacornot"
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/idiom"
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/train"
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
	"idiom":      &idiom.Idiom{},
	"traintimes": &train.Train{},
	"drogslang":  &drogslang.Drogslang{},
}
