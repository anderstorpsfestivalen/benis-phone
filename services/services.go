package services

import (
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/drogslang"
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/train"
)

type Service interface {
	Get(input string, template string, arguments map[string]string) (string, error)
	MaxInputLength() int
}

var ServiceRegistry = map[string]Service{
	// "flacornot":  &flacornot.FlacOrNot{},
	// "barclosing": &barclosing.BarClosing{},
	"traintimes": &train.Train{},
	"drogslang":  &drogslang.Drogslang{},
}
