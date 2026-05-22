package train

import (
	"bytes"
	"fmt"
	"reflect"
	"text/template"

	"github.com/anderstorpsfestivalen/benis-phone/core/secrets"
	"github.com/coral/trafikverket"
	"github.com/coral/trafikverket/responses/trainannouncement"
	"github.com/coral/trafikverket/responses/trainstation"
)

// Args is the typed view of the TOML `args` map for this service.
type Args struct {
	Station string `schema:"default=Reftele" desc:"Trafikverket station name to query departures for"`
}

// TemplateData is the value passed into the caller's text/template. K, From
// and To are external types from the trafikverket package and contain many
// sub-fields not enumerated here — the editor only surfaces Hour, Minute and
// Track.
type TemplateData struct {
	K      trainannouncement.TrainAnnouncement
	From   trainstation.TrainStation
	To     trainstation.TrainStation
	Hour   string `desc:"Two-digit advertised hour, e.g. 14"`
	Minute string `desc:"Two-digit advertised minute, e.g. 07"`
	Track  string `desc:"Spoken-friendly track at location, e.g. \"ett\""`
}

type Train struct{}

func (t *Train) ArgsType() reflect.Type     { return reflect.TypeOf(Args{}) }
func (t *Train) TemplateType() reflect.Type { return reflect.TypeOf(TemplateData{}) }

func (f *Train) Get(input string, tmpl string, arguments map[string]string) (string, error) {
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

			d := TemplateData{
				K:      k,
				From:   from,
				To:     to,
				Hour:   formatted_hour,
				Minute: formatted_minute,
				Track:  k.TrackAtLocation,
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
