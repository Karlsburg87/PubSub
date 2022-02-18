package pubsub

import (
	"fmt"
	"net/http"
	"time"
)

//CreateMux builds the routing for the application. Intended for use with CreateServer
func CreateMux() *http.ServeMux {
	mux := http.NewServeMux()
  //shared mux resources and boot superuser and core struct
  pubsub := getReady("ping","pingpassword")
	//routers - https://pkg.go.dev/net/http#ServeMux
	mux.HandleFunc("/users/create", func(rw http.ResponseWriter, r *http.Request) {
    userCreateHandler(rw,r,pubsub)
  })
	mux.HandleFunc("/subscriptions/subscribe/", func(rw http.ResponseWriter, r *http.Request) {
    subscriptionSubscribeHandler(rw,r,pubsub)
  })
	mux.HandleFunc("/subscriptions/unsubscribe/", func(rw http.ResponseWriter, r *http.Request) {
    subscriptionUnsubscribeHandler(rw,r,pubsub)
  })
	mux.HandleFunc("/topic/create", func(rw http.ResponseWriter, r *http.Request) {
    topicRetrieveHandler(rw,r,pubsub,true)
  })
	mux.HandleFunc("/topic/obtain", func(rw http.ResponseWriter, r *http.Request) {
    topicRetrieveHandler(rw,r,pubsub,false)
  })
	mux.HandleFunc("/message/pull", func(rw http.ResponseWriter, r *http.Request) {
    messagePullHandler(rw,r,pubsub)
  })
	mux.HandleFunc("/message/write", func(rw http.ResponseWriter, r *http.Request) {
    messageWriteHandler(rw,r,pubsub)
  })
  mux.HandleFunc("/",func(rw http.ResponseWriter, r *http.Request) {
    fmt.Fprintln(rw,"Hello World! Welcome to pubsub")
  })
  
	return mux
}

//----------------Handlers

//userCreateHandler creates a new user or returns an existing user if credentials match existing User
func userCreateHandler(rw http.ResponseWriter, r *http.Request,pubsub *PubSub) {
  //login or create user
  user,_,err:=HTTPAuthenticate(rw,r,pubsub) 
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //create response
  response:=CreateUserResp{
    UUID: user.UUID,
  }
  //respond
  respondMuxHTTP(rw,response)
}

//subscriptionSubscribeHandler handles Subscription sign-ups for topics
func subscriptionSubscribeHandler(rw http.ResponseWriter, r *http.Request,pubsub *PubSub) {
  //login user
  user,payload,err:=HTTPAuthenticate(rw,r,pubsub)
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //get topic
  topic,err:=pubsub.GetTopic(payload.Topic,user)
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //subscribe
  err=user.Subscribe(topic,payload.WebhookURL)
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //create response
  response:=SubscribeResp{
    User: user.UUID,
    Topic: topic.Name,
    Status: "Subscribed",
  }
  //respond
  respondMuxHTTP(rw,response) 
}

//subscriptionUnsubscribeHandler handles Subscription withdrawls for topics
func subscriptionUnsubscribeHandler(rw http.ResponseWriter, r *http.Request,pubsub *PubSub) {
  //login user
  user,payload,err:=HTTPAuthenticate(rw,r,pubsub)
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //get topic
  topic,err:=pubsub.GetTopic(payload.Topic,user)
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //unsubscribe
  err=user.Unsubscribe(topic)
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //create response
  response:=SubscribeResp{
    User: user.UUID,
    Topic: topic.Name,
    Status: "Unubscribed",
  }
  
  //respond
  respondMuxHTTP(rw,response)  
}

//topicRetrieveHandler handles create and get requests for topics. If createOnly is true, it will response with an error code if the topic already exists. Otherwise it will create and return the topic. 
func topicRetrieveHandler(rw http.ResponseWriter, r *http.Request,pubsub *PubSub,createOnly bool) {
  //login user
  user,payload,err:=HTTPAuthenticate(rw,r,pubsub)
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //create topic
  var topic *Topic
  if createOnly{
    topic,err=pubsub.CreateTopic(payload.Topic,user)
  }else{
    topic,err=pubsub.GetTopic(payload.Topic,user)
  }
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //create response
  response:=TopicResp{
    Topic: topic.Name,
    Status: "Active",
    PointerHead: topic.PointerHead,
    Creator: topic.Creator.UsernameHash,
  }
  //respond
  respondMuxHTTP(rw,response)
}

//messagePullHandler managers responses to manual http requests for a message. 
// Only works for subscribers that have not got a WebhookURL for  push messages
func messagePullHandler(rw http.ResponseWriter, r *http.Request,pubsub *PubSub) {
  //login user
  user,payload,err:=HTTPAuthenticate(rw,r,pubsub)
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //get topic
  topic,err:=pubsub.GetTopic(payload.Topic,user)
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //pull message
  msg,err:=user.PullMessage(topic,payload.MessageID)
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //create response
  response:=MessageResp{
    Topic: topic.Name,
    Message: msg,
  }
  //respond
  respondMuxHTTP(rw,response) 
}

//messageWriteHandler deals with requests to write messages to a topic. Only the topic creator User is permitted to write to a topic
func messageWriteHandler(rw http.ResponseWriter, r *http.Request,pubsub *PubSub) {
  //login user
  user,payload,err:=HTTPAuthenticate(rw,r,pubsub)
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //get topic
  topic,err:=pubsub.GetTopic(payload.Topic,user)
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //prepare the message
  msg:= Message{
    Data: payload.Message,
    Created: time.Now().Format(time.RFC3339),
  }
  //write a message
  message,err:=user.WriteToTopic(topic,msg)
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //create response
  response:= MessageResp{
    Topic: topic.Name,
    Message: message,
  }
  //respond
  respondMuxHTTP(rw,response)  
 }