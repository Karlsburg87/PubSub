package pubsub

import (
	"encoding/json"
	"fmt"
	"log"
)

//SSEDistro is the object that fans out Messages to SSE requesting clients
type SSEDistro struct {
	//Intake is the incoming message chan that needs to be fanned out to the requesters
	Intake chan SSEResponse
	//Requesters has a clientID as key
	Requesters map[string]chan SSEResponse
	//Add channels is the communication of clients looking to be added o the Requesters map to receive live updates
	Add chan SSEAddRequester
	//Cancel receives ClientID and is used to identify clients that have closed connection and needs removing from Requesters map
	Cancel chan string
}

//SSEAddRequester is the struct sent to SSEDistro to add the client to the message updater. Used by mux to register itself for all message streams
type SSEAddRequester struct {
	ID       string //randomstring hash
	Receiver chan SSEResponse
}

//SSEResponse is the object sent to the client and identifies which topic the message came from
type SSEResponse struct {
	TopicName string  `json:"topic_name,omitempty"`
	Message   Message `json:"message"`
}

//String implements stringer to allow for proper formatting for SSE
func (sse SSEResponse) String() string {
	marsh, err := json.Marshal(sse)
	if err != nil {
		log.Panicln(err)
	}
	//set retry to every 2 seconds as standard
	return fmt.Sprintf("retry: 2000\ndata: %s\n\n", string(marsh))
}

//SSENewDistro creates a new SSEDistro
func SSENewDistro() SSEDistro {
	return SSEDistro{
		Intake:     make(chan SSEResponse),
		Requesters: make(map[string]chan SSEResponse),
		Add:        make(chan SSEAddRequester),
		Cancel:     make(chan string),
	}
}

//Routine is the goroutine that fans out Messages to SSE connections and deals with new fanout receipiants and client removals
//
//Fans out ALL messages to SSE clients. Must be filtered at mux.
func (distro *SSEDistro) Routine() {
	for {
		select {
		case client := <-distro.Add:
			distro.Requesters[client.ID] = client.Receiver

		case msg := <-distro.Intake:
			for _, client := range distro.Requesters {
				client <- msg
			}

		case toDelete := <-distro.Cancel:
			delete(distro.Requesters, toDelete)
		}
	}
}
