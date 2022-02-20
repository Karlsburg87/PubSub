package pubsub

import (
	"fmt"
	"net/http"
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
func CreateMux() *http.ServeMux {
	mux := http.NewServeMux()
	//shared mux resources and boot superuser and core struct
	pubsub := getReady("ping", "pingpassword")
	//routers - https://pkg.go.dev/net/http#ServeMux
	mux.HandleFunc("/user/obtain", func(rw http.ResponseWriter, r *http.Request) {
		userCreateHandler(rw, r, pubsub)
	})
	mux.HandleFunc("/topic/subscribe", func(rw http.ResponseWriter, r *http.Request) {
		subscriptionSubscribeHandler(rw, r, pubsub)
	})
	mux.HandleFunc("/topic/unsubscribe", func(rw http.ResponseWriter, r *http.Request) {
		subscriptionUnsubscribeHandler(rw, r, pubsub)
	})
	mux.HandleFunc("/topic/create", func(rw http.ResponseWriter, r *http.Request) {
		topicRetrieveHandler(rw, r, pubsub, createVerb)
	})
	mux.HandleFunc("/topic/fetch", func(rw http.ResponseWriter, r *http.Request) {
		topicRetrieveHandler(rw, r, pubsub, fetchVerb)
	})
	mux.HandleFunc("/topic/obtain", func(rw http.ResponseWriter, r *http.Request) {
		topicRetrieveHandler(rw, r, pubsub, obtainVerb)
	})
	mux.HandleFunc("/topic/messages/pull", func(rw http.ResponseWriter, r *http.Request) {
		messagePullHandler(rw, r, pubsub)
	})
	mux.HandleFunc("/topic/messages/write", func(rw http.ResponseWriter, r *http.Request) {
		messageWriteHandler(rw, r, pubsub)
	})
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		if err := HTTPErrorResponse(fmt.Errorf("Please choose an API endpoint"), http.StatusInternalServerError, rw); err != nil {
			return
		}
	})

	return mux
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
		UUID:          user.UUID,
		Subscriptions: len(user.Subscriptions),
		Created:       user.Created,
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
		CanWrite: user.UUID == topic.Creator.UUID,
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
		CanWrite: user.UUID == topic.Creator.UUID,
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
		Creator:     topic.Creator.UsernameHash,
		CanWrite:    user.UUID == topic.Creator.UUID,
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
