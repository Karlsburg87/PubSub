package pubsub

import (
	"fmt"
	"log"
	"sync"
)

//getReady creates empty pubsub and users instances to begin the application
//
//Boots in the mux
func getReady(superUsername, superUserpassword string) *PubSub {
	//generate special user `ping`
	superUserPing, err := createNewUser(superUsername, superUserpassword)
	if err != nil {
		log.Fatalln(err)
	}
	//echo superuser login to std.out
	log.Printf("Superuser Ping created.\nUUID: %s", superUserPing.UUID)
	//new core
	users := Users{superUserPing.UsernameHash: superUserPing}
	pubsub := &PubSub{
		Topics: make(Topics),
		Users:  users,
		mu:     &sync.RWMutex{},
	}
	//start regular task ticks
	go pubsub.metranome()
	//start persistance layer
	pubsub.persistLayer, err = NewUnderwriter(pubsub)
	superUserPing.persistLayer = pubsub.persistLayer
	if err != nil {
		log.Fatalln(fmt.Errorf("Error spinning up new Underwriter object: %v", err))
	}

	if err := pubsub.persistLayer.Launch(); err != nil {
		log.Panicln(err)
	}

	//retore existing messages if any in persist locations
	if err := restore(pubsub, pubsub.persistLayer); err != nil {
		log.Panicf("Cannot restore from persist area: %v\n", err) //Fundemental so crash if issue
	}

	return pubsub
}
