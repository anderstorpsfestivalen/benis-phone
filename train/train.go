package train

import (
	"flag"
	"fmt"
	"os"

	"github.com/coral/trafikverket"
)

func Get() string {
	var apikey = os.Getenv("trafiklab_key")
	var searchstation = "Reftele"
	flag.Parse()

	tf := trafikverket.New(apikey)

	station, err := tf.LookupStation(searchstation)
	if err != nil {
		panic(err)
	}

	trann, err := tf.QueryTrainAnnouncementsAtLocationSignature(station.LocationSignature)
	if err != nil {
		panic(err)
	}

	var formatted string

	for _, k := range trann {

		from, err := tf.LookupLocationSignature(k.FromLocation[0].LocationName)
		if err != nil {
			panic(err)
		}

		to, err := tf.LookupLocationSignature(k.ToLocation[0].LocationName)
		if err != nil {
			panic(err)
		}

		formatted_hour := fmt.Sprintf("%02d", k.AdvertisedTimeAtLocation.Hour())
		formatted_minute := fmt.Sprintf("%02d", k.AdvertisedTimeAtLocation.Minute())
		if k.TrackAtLocation == "1" {
			k.TrackAtLocation = "ett"
		}

		formatted = formatted +
			k.InformationOwner + ", " +
			k.ProductInformation[0] + ", " +
			k.TypeOfTraffic + " nummer, " +
			k.TechnicalTrainIdent + ", " +
			"Från " + from.AdvertisedLocationName + ", " +
			"Till " + to.AdvertisedLocationName + ", " +
			"avgår från spår, " + k.TrackAtLocation +
			", klockan, " + formatted_hour +
			", och, " + formatted_minute

	}
	return formatted
}
