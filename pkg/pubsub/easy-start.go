package pubsub

import "log"

//Start is the super easy API from running the PubSub from code
func Start(port int) { //add options struct as second arg
	mux, closer := CreateMux()
	defer closer.Close()
	server := CreateServer(port, mux)

	log.Printf("Server running on port %d\n", port)
	log.Fatalln(server.ListenAndServe())
}
