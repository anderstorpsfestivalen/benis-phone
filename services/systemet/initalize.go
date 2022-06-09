package systemet

import (
	log "github.com/sirupsen/logrus"
	"gitlab.com/anderstorpsfestivalen/benis-phone/services"
)

// This is such a shit hack to retrofit this into the new structure
// FML

func InitalizeServices() {
	err := Init()
	if err != nil {
		log.Error(err)
		log.Fatal("Could not init systembolaget lookup")
	}

	// Setup Systemet API
	key, err := GetKey()
	if err != nil {
		log.Error(err)
		log.Panic("Could not get systembolaget key")
	}

	api := New(key)

	//Add Stock service
	systemetStock := CreateStock(api)
	services.AddService("systemetstock", systemetStock)

	//Add Pid service
	systemetPID := CreatePidSearch(api)
	services.AddService("systemetpid", systemetPID)
}
