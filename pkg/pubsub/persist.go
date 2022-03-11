package pubsub

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

//PersistBase gives the root directory location to which data should be persisted. Set by envar `STORE`
var PersistBase string

func init() {
	PersistBase = os.Getenv("STORE")
	if PersistBase == "" {
		PersistBase = "store/"
	}
}

//PersistCore is the minimum fields an implementor of Persist should have
type PersistCore struct {
	pubsub *PubSub

	userWriter       chan User                    //userWriter used for saving User data in persistent layer
	subscriberWriter chan PersistSubscriberStruct //subscriberWriter chan to persist layer for saving
	messageWriter    chan PersistMessageStruct    // messageWriter chan to persist layer for saving

	userDeleter       chan string                  //userDeleter takes a user.UUID as input
	subscriberDeleter chan PersistSubscriberStruct //subscriberDeleter takes data for sub deletion
	messageDeleter    chan PersistMessageStruct    //messageDeleter takes data for msg deletion
}

//Persist is the interface for adding persistent storage
type Persist interface {
	//Launch starts up goroutines
	Launch() error
	//OutputSwitchboard returns a PersistCore object
	// within which to send messages to Write...
	// and Delete... methods
	Switchboard() PersistCore
	//TidyUp is where to put the close down work. Usually
	// to close
	// files or databases. Usually used as `defer
	// Persist.TidyUp()`
	TidyUp() error
	//WriteUser adds a user to the persistence layer from
	// a User chan
	WriteUser() error
	//WriteSubscriber adds a subscriber to the persistence
	// layer from persistSubscriberStruct chan
	WriteSubscriber() error
	//WriteMessage adds a message to the persistence layer
	// with from persistMessageStruct
	WriteMessage() error
	//GetUseret returns a single user by userID string
	// (Which is also UsernameHash of the user)
	GetUser(string) (User, error)
	//GetSubscriberet returns a single subscriber by
	// subcriberID (which is also the userID attachted to
	// the subscriber),messageID and topicName
	GetSubscriber(string, int, string) (Subscriber, error)
	//GetMessage returns a single message by messageID and topicName
	GetMessage(int, string) (Message, error)
	//StreamUsers returns a chan through which it streams
	// all Users from the db
	StreamUsers() (chan Streamer, error)
	//StreamSubscribers returns a chan through which it streams all
	// Subscribers from the db
	StreamSubscribers() (chan Streamer, error)
	//StreamMessages returns a chan through which it streams all
	// Messages from the db
	StreamMessages() (chan Streamer, error)
	//DeleteUser accepts UserID which is the userhash string
	DeleteUser() error
	//DeleteSubscriber accepts subscriberID (the userID of the subscription),
	// topicName.
	//
	//Add messageID as -1 if not available. Func will they cycle through the topic and delete matches to subscriberID
	DeleteSubscriber() error
	//DeleteMessage accepts messageID and topicName
	DeleteMessage() error
}

//Streamer is the response object from Stream restore methods of
// the Persist interface
type Streamer struct {
	//Unit is User, Subscriber or Message. Topic can be
	// passed but is implicitly available in the Key
	Unit tombstoner
	//Key is the db key which contains id information for
	// proper restoration to active map
	//
	//In the format order (curly bracketed items are optional, square bracketed items are Unit type dependant):
	// {bucketName/}TopicName/MessageID[/SubscriberID]
	//or just:
	// {bucketName/}UserID
	Key string
}

//PersistSubscriberStruct is a channel object for sending // messages to be saved by the persist layer
type PersistSubscriberStruct struct {
	Subscriber   Subscriber //for saving
	MessageID    int
	TopicName    string
	SubscriberID string //for deletions
}

//PersistMessageStruct is a channel object for sending // messages to be saved by the persist layer
type PersistMessageStruct struct {
	Message   Message //for saving
	TopicName string
	MessageID int //for deletions
}

//restoreUsers is a component of restore function
func restoreUsers(ping *User, pubsub *PubSub, persist Persist) error {
	//restore users first
	uStream, err := persist.StreamUsers()
	if err != nil {
		return err
	}
	for userShell := range uStream {
		usr, ok := userShell.Unit.(*User)
		if !ok {
			return fmt.Errorf("StreamUsers did not return *User")
		}
		usr.mu = &sync.RWMutex{}
		usr.persistLayer = pubsub.persistLayer
		pubsub.Users[userShell.Key] = usr
	}
	return nil
}

//restoreMessages is a component of restore function
func restoreMessages(ping *User, pubsub *PubSub, persist Persist) error {
	mStream, err := persist.StreamMessages()
	if err != nil {
		return err
	}
	for messageShell := range mStream {
		msg, ok := messageShell.Unit.(*Message)
		if !ok {
			return fmt.Errorf("StreamMessages did not return *Message")
		}
		pieces := strings.Split(messageShell.Key, "/")
		fileName := pieces[len(pieces)-1]
		msgID, err := strconv.Atoi(fileName[:len(fileName)-len(path.Ext(fileName))])
		if err != nil {
			return err
		}
		fmt.Println(pieces)
		topicName := pieces[len(pieces)-2]
		//Create Topic if not yet existing
		if _, ok := pubsub.Topics[topicName]; !ok {
			pubsub.CreateTopic(topicName, ping) //need userID which is in subscriber.Creator -> bool. So default to `ping' and have this updated when restoring subscriptions.
		}
		pubsub.Topics[topicName].Messages[msgID] = *msg
		//Update topic's' pointerHead
		if pubsub.Topics[topicName].PointerHead <= (msgID + 1) {
			pubsub.Topics[topicName].PointerHead = (msgID + 1)
		}
	}
	return nil
}

//restoreSubscriptions is a component of restore function
func restoreSubscriptions(ping *User, pubsub *PubSub, persist Persist) error {
	sStream, err := persist.StreamSubscribers()
	if err != nil {
		return err
	}
	for subShell := range sStream {
		sub, ok := subShell.Unit.(*Subscriber)
		if !ok {
			return fmt.Errorf("StreamSubscribers did not return *Subscriber")
		}
		sub.mu = &sync.RWMutex{}
		pieces := strings.Split(subShell.Key, "/")
		subID := pieces[len(pieces)-1]
		msgID, err := strconv.Atoi(pieces[len(pieces)-2])
		if err != nil {
			return err
		}
		topicName := pieces[len(pieces)-3]

		if _, ok := pubsub.Topics[topicName].PointerPositions[msgID]; !ok {
			pubsub.Topics[topicName].PointerPositions[msgID] = make(Subscribers)
		}
		pubsub.Topics[topicName].PointerPositions[msgID][subID] = sub

		//restore as creator of Topic if marked on subscription and not the default ping
		if sub.Creator && sub.ID != ping.UUID {
			pubsub.Topics[topicName].Creator = sub.ID
		}
	}
	return nil
}

//restore reinstates a snapshot back to memory if it exists
func restore(pubsub *PubSub, persist Persist) error {
	//get ping superuser as default Topic creator
	var ping *User
	for _, user := range pubsub.Users {
		ping = user
		break //ping should be first and only User at startup
	}
	//restore users first
	if err := restoreUsers(ping, pubsub, persist); err != nil {
		return err
	}
	//restore messages second (and implicitly Topics)
	if err := restoreMessages(ping, pubsub, persist); err != nil {
		return err
	}
	//restore subscriptions last
	if err := restoreSubscriptions(ping, pubsub, persist); err != nil {
		return err
	}

	return nil
}
