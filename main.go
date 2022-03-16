package main

import (
	"log"
	"os"
	"strconv"

	"github.com/CDennis-CR/PubSub/pkg/pubsub"
)

//start pubsub server
func main() {
	//start service
	pubsub.Start(getPortFromEnvar("PS_PORT", 8080))
}

func getPortFromEnvar(variableName string, defaultPort int) int {
	//get port from environment variables
	portS := os.Getenv("variableName")
	var port int
	var err error

	if portS == "" {
		port = 8080
	} else {
		port, err = strconv.Atoi(portS)
		if err != nil {
			log.Fatalln(err)
		}
	}
	return port
}
