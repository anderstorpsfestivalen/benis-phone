package train

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/anderstorpsfestivalen/benis-phone/core/secrets"
	"github.com/coral/trafikverket"
	"github.com/coral/trafikverket/responses/trainannouncement"
	"github.com/coral/trafikverket/responses/trainstation"
)

type Train struct{}

func (f *Train) Get(input string, tmpl string, arguments map[string]string) (string, error) {
	type Times struct {
		K      trainannouncement.TrainAnnouncement
		From   trainstation.TrainStation
		To     trainstation.TrainStation
		Hour   string
		Minute string
	}

	ttmpl, err := template.New("traintimes").Parse(tmpl)
	if err != nil {
		return "", err
	}

	credentials := secrets.Loaded

	var searchstation = "Reftele"

	//check for station argument
	if v, ok := arguments["station"]; ok {
		searchstation = v
	}

	tf := trafikverket.New(credentials.Trafiklab)

	station, err := tf.LookupStation(searchstation)
	if err != nil {
		return "", err
	}

	trann, err := tf.QueryTrainAnnouncementsAtLocationSignature(station.LocationSignature)
	if err != nil {
		return "", err
	}

	var formatted string

	for i, k := range trann {
		if i < 1 {

			from, err := tf.LookupLocationSignature(k.FromLocation[0].LocationName)
			if err != nil {
				return "", err
			}

			to, err := tf.LookupLocationSignature(k.ToLocation[0].LocationName)
			if err != nil {
				return "", err
			}

			formatted_hour := fmt.Sprintf("%02d", k.AdvertisedTimeAtLocation.Hour())
			formatted_minute := fmt.Sprintf("%02d", k.AdvertisedTimeAtLocation.Minute())
			if k.TrackAtLocation == "1" {
				k.TrackAtLocation = "ett"
			}

			d := Times{
				K:      k,
				From:   from,
				To:     to,
				Hour:   formatted_hour,
				Minute: formatted_minute,
			}

			buf := new(bytes.Buffer)
			err = ttmpl.Execute(buf, d)
			if err != nil {
				return "", err
			}

			formatted = formatted + buf.String()
		}
	}
	return formatted, nil
}

func (t *Train) MaxInputLength() int {
	return 0
}
