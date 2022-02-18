package pubsub

import (
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
		mu:     &sync.Mutex{},
	}
	//start regular task ticks
	go pubsub.metranome()

	return pubsub
}
