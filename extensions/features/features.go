package feature

import (
	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/features/atpqueue"
)

// Hard coded functionality that is hard to generalize

type Feature interface {
	Start(audio *audio.Audio)
	Input(key string)
	Stop()
}

/* var FeatureRegistry = map[string]Feature{
	"atpqueue": &atpqueue.ATPQueue{},
} */

var FeatureRegistry = map[string]Feature{
	"atpqueue": &atpqueue.ATPQueue{},
}
