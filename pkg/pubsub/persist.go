package pubsub

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

//PersistBase gives the root directory location to which data should be persisted. Set by envar `STORE`
var PersistBase string

func init() {
	PersistBase = os.Getenv("STORE")
	if PersistBase == "" {
		PersistBase = "store/"
	}
}

//Persist is the interface for adding persistant storage
type Persist interface {
	//TidyUp is where to put the close down work. Usually to close
	// files or databases. Usually used as `defer Persist.TidyUp()`
	TidyUp() error
	//WriteUser adds a user to the persistance layer
	WriteUser(*User) error
	//WriteSubscriber adds a subscriber to the persistance layer
	WriteSubscriber(*Subscriber, Message, *Topic) error
	//WriteMessage adds a message to the persistance layer
	WriteMessage(Message, *Topic) error
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
	DeleteUser(string) error
	//DeleteSubscriber accepts subscriberID (the userID of the subscription), messageID and topicName
	DeleteSubscriber(string, int, string) error
	//DeleteMessage accepts messageID and topicName
	DeleteMessage(int, string) error
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

//restore reinstates a snapshot back to memory if it exists
func restore(pubsub *PubSub, persist Persist) error {
	//get ping superuser as default Topic creator
	var ping *User
	for _, user := range pubsub.Users {
		ping = user
		break
	}
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
		pubsub.Users[userShell.Key] = usr
	}
	//restore messages second (and implicitly Topics)
	mStream, err := persist.StreamMessages()
	for messageShell := range mStream {
		msg, ok := messageShell.Unit.(*Message)
		if !ok {
			return fmt.Errorf("StreamMessages did not return *Message")
		}
		pieces := strings.Split(messageShell.Key, "/")
		msgID, err := strconv.Atoi(pieces[len(pieces)-1])
		if err != nil {
			return err
		}
		topicName := pieces[len(pieces)-2]
		//Create Topic if not yet existing
		if _, ok := pubsub.Topics[topicName]; !ok {
			pubsub.CreateTopic(topicName, ping) //need userID which is in subscriber.Creator -> bool. So default to `ping' and have this updated when restoring subscriptions.
		}
		pubsub.Topics[topicName].Messages[msgID] = *msg
	}
	//restore subscriptions last
	sStream, err := persist.StreamSubscribers()
	if err != nil {
		return err
	}
	for subShell := range sStream {
		sub, ok := subShell.Unit.(*Subscriber)
		if !ok {
			return fmt.Errorf("StreamSubscribers did not return *Subscriber")
		}
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

		//restore as creator of Topic if so
		if sub.Creator {
			pubsub.Topics[topicName].Creator = sub.User
		}
	}

	return nil
}
