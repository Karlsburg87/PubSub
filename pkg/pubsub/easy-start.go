package pubsub

import "log"

//Start is the super easy API from running the PubSub from code
func Start(port int) { //add options struct as second arg
	mux, closer := CreateMux(MuxAPI, nil)
	defer closer.Close()
	server := CreateServer(port, ServerAPI, mux)

	//start SSE server seperately in goroutine
	go func(pubsub *PubSub) {
		ssePort := 4039

		sseMux, _ := CreateMux(MuxSSE, closer)
		sseServer := CreateServer(ssePort, ServerSSE, sseMux)

		log.Printf("SSE server running on port %d\n", ssePort)
		log.Fatalln(sseServer.ListenAndServe())
	}(closer)

	//start API server in main thread
	log.Printf("Server running on port %d\n", port)
	log.Fatalln(server.ListenAndServe())
}
