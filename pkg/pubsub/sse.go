package pubsub

//SSEDistro is the object that fans out Messages to SSE requesting clients
type SSEDistro struct {
	//Intake is the incoming message chan that needs to be fanned out to the requesters
	Intake     chan SSEResponse            //Intake is the chan messages are sent before fanout to SSE connections
	Requesters map[string]chan SSEResponse //Requesters has a clientID as key
	Add        chan SSEAddRequester
	Cancel     chan string //Cancel receives ClientID
}

//SSEAddRequester is the struct sent to SSEDistro to add the client to the message updater. Used by mux to register itself for all message streams
type SSEAddRequester struct {
	ID       string //userIP+randomstring hash
	Receiver chan SSEResponse
}

//SSEResponse is the object sent to the client and identifies which topic the message came from
type SSEResponse struct {
	TopicName string  `json:"topic_name,omitempty"`
	Message   Message `json:"message"`
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
