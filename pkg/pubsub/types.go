package pubsub

import (
	"sync"
	"time"
)

//PubSub is the core holder struct for the pubsub service
type PubSub struct {
	Topics Topics
	Users  Users
	mu     *sync.RWMutex
	//persistLayer is the data persistence interface
	persistLayer Persist
	//sseDistro is unit that supplies Server Sent Events to the mux frontend
	sseDistro SSEDistro
}

//Topic is the setup for topics
type Topic struct {
	Creator          string              //Creater is a User ID for the creator user. Only User that can write to the topic
	Name             string              //user given name for the topic (sanitized)
	Messages         map[int]Message     //message queue
	PointerPositions map[int]Subscribers //pointer position against subscribers at that position
	PointerHead      int                 //latest/highest Messages key/ID.
	mu               *sync.RWMutex
	tombstone        string //timestamp - deleted in 10 minutes
	//sseOut is the channel ALL messages can be sent to to fan out to sse requests
	sseOut chan SSEResponse
}

//Topics is a map of topics with key as topic name
type Topics map[string]*Topic

//Subscriber is the setup of a subscriber to a topic
type Subscriber struct {
	ID              string //ID is the User.UUID
	UsernameHash    string //UsernameHash is the User.UsernameHash to help access the user in Subscription based functions
	PushURL         string //PushURL is the webhook URL to which to push messages
	mu              *sync.RWMutex
	tombstone       string        //tombstone is a timestamp - deleted in 10 minutes
	lastpushAttempt time.Time     //lastpushAttempt is the last attempt to push the message via webhook
	backoff         time.Duration //backoff duration to allow for exponential backoff
	Creator         bool          //Creator is whether or not the subscriber is the creator. Used for `restore`
}

//Subscribers is a map of subscribers
type Subscribers map[string]*Subscriber //Subscriber.ID against subscriber

//Message is a single message structure
type Message struct {
	ID        int         `json:"id"` //sequence number
	Data      interface{} `json:"data"`
	Created   string      `json:"created"`
	tombstone string      //timestamp - deleted in 10 minutes
}

//User is the struct of a user able to make a subscription
type User struct {
	UUID          string //hash of Username+Password
	UsernameHash  string
	PasswordHash  string
	Subscriptions map[string]string //Topic Names key against pushURL
	Created       string            //Created is date user was created
	mu            *sync.RWMutex
	tombstone     string //timestamp - deleted in 10 minutes
	//persistLayer is the data persistence interface
	persistLayer Persist
}

//Users is a map of User by "UsernameHash" key with value of User
type Users map[string]*User
