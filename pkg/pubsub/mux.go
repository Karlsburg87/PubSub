package pubsub

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

//CreateMux builds the routing for the application. Intended for use with CreateServer
//
//Verbs ------
//
//Obtain : Get existing or create
//
//Create : Create new or error if already exists
//
//Fetch  : Get existing or error if does not exists
//
//Write  : Write data to server
//
//Pull   : Read information from server by http request (pull) after subscribing to a pull agreement of event data
//
//Subscribe/Unsubscribe : Setup or delete push/pull agreement
func CreateMux() (*http.ServeMux, *PubSub) {
	mux := http.NewServeMux()
	//shared mux resources and boot superuser and core struct
	pubsub := getReady("ping", "pingpassword")
	//routers - https://pkg.go.dev/net/http#ServeMux
	mux.HandleFunc("/users/user/obtain", func(rw http.ResponseWriter, r *http.Request) {
		userCreateHandler(rw, r, pubsub)
	})
	mux.HandleFunc("/topics/topic/subscribe", func(rw http.ResponseWriter, r *http.Request) {
		subscriptionSubscribeHandler(rw, r, pubsub)
	})
	mux.HandleFunc("/topics/topic/unsubscribe", func(rw http.ResponseWriter, r *http.Request) {
		subscriptionUnsubscribeHandler(rw, r, pubsub)
	})
	mux.HandleFunc("/topics/fetch", func(rw http.ResponseWriter, r *http.Request) {
		topicsListHandler(rw, r, pubsub)
	})
	mux.HandleFunc("/topics/topic/create", func(rw http.ResponseWriter, r *http.Request) {
		topicRetrieveHandler(rw, r, pubsub, createVerb)
	})
	mux.HandleFunc("/topics/topic/fetch", func(rw http.ResponseWriter, r *http.Request) {
		topicRetrieveHandler(rw, r, pubsub, fetchVerb)
	})
	mux.HandleFunc("/topics/topic/obtain", func(rw http.ResponseWriter, r *http.Request) {
		topicRetrieveHandler(rw, r, pubsub, obtainVerb)
	})
	mux.HandleFunc("/topics/topic/messages/pull", func(rw http.ResponseWriter, r *http.Request) {
		messagePullHandler(rw, r, pubsub)
	})
	mux.HandleFunc("/topics/topic/messages/write", func(rw http.ResponseWriter, r *http.Request) {
		messageWriteHandler(rw, r, pubsub)
	})
	//UI based routes
	mux.HandleFunc("/sse", func(rw http.ResponseWriter, r *http.Request) {
		sseHandler(rw, r, pubsub)
	})
	mux.HandleFunc("/", homepageHandler)

	return mux, pubsub
}

//----------------Handlers

//userCreateHandler creates a new user or returns an existing user if credentials match existing User
func userCreateHandler(rw http.ResponseWriter, r *http.Request, pubsub *PubSub) {
	//login or create user
	user, _, err := HTTPAuthenticate(rw, r, pubsub)
	if err != nil {
		return
	}
	//create response
	response := CreateUserResp{
		UUID:              user.UUID,
		SubscriptionCount: len(user.Subscriptions),
		Subscriptions:     user.Subscriptions,
		Created:           user.Created,
	}
	//respond
	respondMuxHTTP(rw, response)
}

//subscriptionSubscribeHandler handles Subscription sign-ups for topics
func subscriptionSubscribeHandler(rw http.ResponseWriter, r *http.Request, pubsub *PubSub) {
	//login user
	user, payload, err := HTTPAuthenticate(rw, r, pubsub)
	if err != nil {
		return
	}
	//get topic
	topic, err := pubsub.GetTopic(payload.Topic, user)
	if err := HTTPErrorResponse(err, http.StatusInternalServerError, rw); err != nil {
		return
	}
	//subscribe
	err = user.Subscribe(topic, payload.WebhookURL)
	if err := HTTPErrorResponse(err, http.StatusInternalServerError, rw); err != nil {
		return
	}
	//create response
	response := SubscribeResp{
		User:     user.UUID,
		Topic:    topic.Name,
		Status:   "Subscribed",
		CanWrite: user.UUID == topic.Creator,
	}
	//respond
	respondMuxHTTP(rw, response)
}

//subscriptionUnsubscribeHandler handles Subscription withdrawls for topics
func subscriptionUnsubscribeHandler(rw http.ResponseWriter, r *http.Request, pubsub *PubSub) {
	//login user
	user, payload, err := HTTPAuthenticate(rw, r, pubsub)
	if err != nil {
		return
	}
	//get topic
	topic, err := pubsub.FetchTopic(payload.Topic, user)
	if err := HTTPErrorResponse(err, http.StatusInternalServerError, rw); err != nil {
		return
	}
	//unsubscribe
	err = user.Unsubscribe(topic)
	if err := HTTPErrorResponse(err, http.StatusInternalServerError, rw); err != nil {
		return
	}
	//create response
	response := SubscribeResp{
		User:     user.UUID,
		Topic:    topic.Name,
		Status:   "Unsubscribed",
		CanWrite: user.UUID == topic.Creator,
	}

	//respond
	respondMuxHTTP(rw, response)
}

//topicsListHandler handles fetch requests for a list of available topics to subscribe topic
func topicsListHandler(rw http.ResponseWriter, r *http.Request, pubsub *PubSub) {
	//login user
	_, _, err := HTTPAuthenticate(rw, r, pubsub)
	if err != nil {
		return
	}
	//get list
	list := make([]string, 0, len(pubsub.Topics))
	for k := range pubsub.Topics {
		list = append(list, k)
	}
	//create response
	response := ListKeysResp{
		Topics: list,
		Count:  len(list),
	}
	//respond
	respondMuxHTTP(rw, response)
}

//topicRetrieveHandler handles create and get requests for topics. If createOnly is true, it will response with an error code if the topic already exists. Otherwise it will create and return the topic.
func topicRetrieveHandler(rw http.ResponseWriter, r *http.Request, pubsub *PubSub, verb verbType) {
	//login user
	user, payload, err := HTTPAuthenticate(rw, r, pubsub)
	if err != nil {
		return
	}
	//create topic
	var topic *Topic
	switch verb {
	case createVerb:
		topic, err = pubsub.CreateTopic(payload.Topic, user)
	case fetchVerb:
		topic, err = pubsub.FetchTopic(payload.Topic, user)
	default: //obtainVerb
		topic, err = pubsub.GetTopic(payload.Topic, user)
	}
	if err := HTTPErrorResponse(err, http.StatusInternalServerError, rw); err != nil {
		return
	}
	//create response
	response := TopicResp{
		Topic:       topic.Name,
		Status:      "Active",
		PointerHead: topic.PointerHead,
		Creator:     topic.Creator,
		CanWrite:    user.UUID == topic.Creator,
	}
	//respond
	respondMuxHTTP(rw, response)
}

//messagePullHandler managers responses to manual http requests for a message.
// Only works for subscribers that have not got a WebhookURL for  push messages
func messagePullHandler(rw http.ResponseWriter, r *http.Request, pubsub *PubSub) {
	//login user
	user, payload, err := HTTPAuthenticate(rw, r, pubsub)
	if err != nil {
		return
	}
	//get topic
	topic, err := pubsub.GetTopic(payload.Topic, user)
	if err := HTTPErrorResponse(err, http.StatusInternalServerError, rw); err != nil {
		return
	}
	//pull message
	msg, err := user.PullMessage(topic, payload.MessageID)
	if err := HTTPErrorResponse(err, http.StatusInternalServerError, rw); err != nil {
		return
	}
	//create response
	response := MessageResp{
		Topic:   topic.Name,
		Message: msg,
	}
	//respond
	respondMuxHTTP(rw, response)
}

//messageWriteHandler deals with requests to write messages to a topic. Only the topic creator User is permitted to write to a topic
func messageWriteHandler(rw http.ResponseWriter, r *http.Request, pubsub *PubSub) {
	//login user
	user, payload, err := HTTPAuthenticate(rw, r, pubsub)
	if err != nil {
		return
	}
	//get topic
	topic, err := pubsub.GetTopic(payload.Topic, user)
	if err := HTTPErrorResponse(err, http.StatusInternalServerError, rw); err != nil {
		return
	}
	//prepare the message
	msg := Message{Data: payload.Message}
	msg.AddCreatedDatestring(time.Now())
	//write a message
	message, err := user.WriteToTopic(topic, msg)
	if err := HTTPErrorResponse(err, http.StatusInternalServerError, rw); err != nil {
		return
	}
	//create response
	response := MessageResp{
		Topic:   topic.Name,
		Message: message,
	}
	//respond
	respondMuxHTTP(rw, response)
}

//sseHandler is a handler for Server Side Events requests
//
//Useful design pattern for SSE:https://www.smashingmagazine.com/2018/02/sse-websockets-data-flow-http2/#:~:text=Server%2DSent%20Events%20are%20real,communication%20method%20from%20the%20server.
func sseHandler(rw http.ResponseWriter, r *http.Request, pubsub *PubSub) {
	//Set SSE and CORS headers
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")
	//sign-up to streams
	clientName := RandomString(6)
	receiver := make(chan SSEResponse)
	pubsub.sseDistro.Add <- SSEAddRequester{
		ID:       clientName,
		Receiver: receiver,
	}
	//get topic stream filters - expected under topic url query as seperate args under the `topic` key
	filterIn := make(map[string]bool)
	for _, term := range r.URL.Query()["topic"] {
		filterIn[term] = true
	}
	//ensure this doesn't block on client closing the connection
	defer func() {
		pubsub.sseDistro.Cancel <- clientName
	}()
	//Flusher
	var flusher http.Flusher
	if flush, ok := rw.(http.Flusher); ok {
		flusher = flush
	} else {
		log.Panicln("No flusher on rw in SSE")
	}
	//Keep alive pinger
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	//id counter
	idCount := 0
	//Run the SSE loop
	for {
		select {

		case <-r.Context().Done():
			log.Printf("Connection cancelled by client: %s\n", clientName)
			return

		case <-ticker.C:
			if _, err := fmt.Fprint(rw, ": keep alive\n\n"); err != nil {
				log.Println(err)
				return
			}
			flusher.Flush()

		case item := <-receiver:
			//Figure out which topic stream was requested
			if !filterIn[item.TopicName] {
				continue
			}
			//increment the id count
			idCount += 1
			if _, err := fmt.Fprintf(rw, "id: %d\n", idCount); err != nil {
				log.Println(err)
				return
			}
			//item implements stringer with data: and \n\n wrapped
			if _, err := fmt.Fprint(rw, item); err != nil {
				log.Println(err)
				return
			}
			//flush remaining data
			flusher.Flush()

		}
	}
}

//homepageHandler outputs the homepage webapp
func homepageHandler(rw http.ResponseWriter, r *http.Request) {
	//Check if request is for supporting files- .js and .css
	switch r.URL.Path {
	case "/app.js":
		http.ServeFile(rw, r, "webapp/app.js")
		return
	case "/app.css":
		http.ServeFile(rw, r, "webapp/app.css")
		return
	}

	//serve homepage via serveContent to allow for templating
	file, err := os.Open("webapp/app.html")
	if err := HTTPErrorResponse(err, http.StatusInternalServerError, rw); err != nil {
		return
	}
	defer file.Close()
	http.ServeContent(rw, r, "index.html", time.Time{}, file)
}
