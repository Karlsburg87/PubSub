package main

import (
	"log"
  "main/pkg/pubsub" //For running in Replit during dev
)

//start pubsub server
func main() {
	port := 8080
  
	server := pubsub.CreateServer(port, pubsub.CreateMux())

	log.Printf("Server running on port %d\n", port)
	log.Fatalln(server.ListenAndServe())
}
