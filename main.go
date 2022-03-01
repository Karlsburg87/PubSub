package main

import (
	"github.com/CDennis-CR/PubSub/pkg/pubsub"
)

//start pubsub server
func main() {
	port := 8080

pubsub.Start(port)
}
