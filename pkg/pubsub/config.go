package pubsub

import (
	"log"
	"time"
)

var (
	durationToStale      time.Duration //durationToStale is the time allowed before an orphaned object is tombstoned
	durationForResurrect time.Duration //durationForResurrect is the time allowed after tombstoning before a deletion is committed
	adminUsername        string        //adminUsername is the username of the initial system user
	adminPassword        string        // adminPassword is the password of the initial system user
	//persistToDirPath gives the root directory location to which data should be persisted. Set by envar `PS_STORE`
	persistToDirPath string
)

func init() {
	var err error
	dTS := envarOrDefault("PS_DURATION_STALE", "3h")
	durationToStale, err = time.ParseDuration(dTS)
	if err != nil {
		log.Fatalln(err)
	}
	dFR := envarOrDefault("PS_DURATION_RESURRECT", "30m")
	durationForResurrect, err = time.ParseDuration(dFR)
	if err != nil {
		log.Fatalln(err)
	}
	adminUsername = envarOrDefault("PS_SUPERADMIN_USERNAME", "ping")
	adminPassword = envarOrDefault("PS_SUPERADMIN_PASSWORD", RandomString(6))
	persistToDirPath = envarOrDefault("PS_STORE", "store/")
}
