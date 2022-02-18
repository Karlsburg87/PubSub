package main

import (
	"github.com/CDennis-CR/PubSub/pkg/pubsub"
	"log"
)

//start pubsub server
func main() {
	port := 8080

	server := pubsub.CreateServer(port, pubsub.CreateMux())

	log.Printf("Server running on port %d\n", port)
	log.Fatalln(server.ListenAndServe())
}
