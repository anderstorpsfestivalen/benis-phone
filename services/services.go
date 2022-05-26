package services

import (
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/barclosing"
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/flacornot"
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/train"
)

type Service interface {
	Get(string) (string, error)
}

var ServiceRegistry = map[string]Service{
	"flacornot":  &flacornot.FlacOrNot{},
	"barclosing": &barclosing.BarClosing{},
	"traintimes": &train.Train{},
}
