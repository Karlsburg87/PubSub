package pubsub

import (
	"net/url"
	"sync"
)

//PubSub is the core holder struct for the pubsub service
type PubSub struct {
	Topics Topics
	Users  Users
	mu     *sync.Mutex
}

//Topic is the setup for topics
type Topic struct {
	ID               int                 //sequential number--unused: safe to delete--
	Creator          *User               //only User that can write to the topic
	Name             string              //user given name for the topic (sanitized)
	Messages         map[int]Message     //message queue
	PointerPositions map[int]Subscribers //pointer position against subscribers at that position
	PointerHead      int                 //latest/highest Messages key/ID.
}

//Topics is a map of topics with key as topic name
type Topics map[string]*Topic

//Subscriber is the setup of a subscriber to a topic
type Subscriber struct {
	ID      string //User.UUID
	User    *User
	PushURL *url.URL
}

//Subscribers is a map of subscribers
type Subscribers map[string]Subscriber //Subscriber.ID against subscriber

//Message is a single message structure
type Message struct {
	ID      int         `json:"id"` //sequence number
	Data    interface{} `json:"data"`
	Created string      `json:"created"`
}

//User is the struct of a user able to make a subscription
type User struct {
	UUID          string //hash of Username+Password
	UsernameHash  string
	PasswordHash  string
	Subscriptions map[int]string //Topic IDs key against pushURL
	mu            *sync.Mutex
}

//Users is a map of User by "UsernameHash" key with value of User
type Users map[string]*User
