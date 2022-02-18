package pubsub

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"
)

//GetUser maintains the user list
//
//Returns existing user record if usernameHash and passwordHash match
// Otherwise creates new user if no match or return login
// error if password is no match to existing user with same username
func (pubsub *PubSub) GetUser(username, password string) (*User, error) {
	user, err := createNewUser(username, password)
	if err != nil {
		return nil, err
	}
	if rec, ok := pubsub.Users[user.UsernameHash]; ok { //username found so check password...
		if user.PasswordHash == rec.PasswordHash { //password correct so return User
			return rec, nil
		}
		//password incorrect return error
		return nil, fmt.Errorf("User already exists. Please enter correct credentials to login or select a new username to create a new user.")

	} else { //create user if no username exists
		pubsub.mu.Lock()
		pubsub.Users[user.UsernameHash] = user
		pubsub.mu.Unlock()
		return pubsub.Users[user.UsernameHash], nil
	}
}

//GetTopic gets a topic. If it does not exist it creates a new topic
// using the User as the creator
func (pubsub *PubSub) GetTopic(topicName string, user *User) (*Topic, error) {
	if topic, ok := pubsub.Topics[topicName]; ok {
		return topic, nil
	}
	return pubsub.CreateTopic(topicName, user)
}

//CreateTopic creates a topic
func (pubsub *PubSub) CreateTopic(topicName string, user *User) (*Topic, error) {
	//Return error if the topic already exists
	if _, ok := pubsub.Topics[topicName]; ok {
		return nil, fmt.Errorf("Topic already exists")
	}
	newTopic := &Topic{
		Creator:          user,
		Name:             topicName,
		PointerHead:      0,
		PointerPositions: make(map[int]Subscribers),
		Messages:         make(map[int]Message),
	}
	//Add the topic to the public topic list
	pubsub.mu.Lock()
	pubsub.Topics[newTopic.Name] = newTopic
	//subscribe the User
	p := pubsub.Topics[topicName]
	user.Subscribe(p, "")
	pubsub.mu.Unlock()

	return p, nil
}

//PushWebhooks runs through all topics and pushes messages to
// the subscribers as a Webhook service
//
//Should run through continuously
func (pubsub *PubSub) PushWebhooks() error {
	//cycle through Topics
	for topicID, topic := range pubsub.Topics {
		//cycle through Messages
		for msgID, message := range topic.Messages {
			//send message to all push subscribers
			for subscriberID, subscriber := range topic.PointerPositions[msgID] {
				if subscriber.PushURL != nil {
					//push to url and await for 200/201 acknolegement
					msgParcel := MessageResp{
						Topic:   topicID,
						Message: message,
					}
					parcel, err := msgParcel.toJSON()
					if err != nil {
						return err
					} //?echo err and continue?
					resp, err := http.Post(subscriber.PushURL.String(), "application/json", bytes.NewReader(parcel))
					if err != nil || (resp.StatusCode != 200 && resp.StatusCode != 201) {
						log.Println(fmt.Errorf("Could not deliver msg: %v", err))
						continue
					}
					//push subscriber pointer up an index place
					pubsub.mu.Lock()
					pubsub.Topics[topicID].PointerPositions[msgID+1][subscriberID] = pubsub.Topics[topicID].PointerPositions[msgID][subscriberID] //add new position
					delete(pubsub.Topics[topicID].PointerPositions[msgID], subscriberID)                                                          //delete old position
					pubsub.mu.Unlock()
				}
			}
		}
	}
	return nil
}

//metranome initiates regularly occuring activities
// such as fullfilling push subscriptions, backing up
// and garbage collection of messages
func (pubsub *PubSub) metranome() {
	t := time.Tick(1 * time.Second)

	for range t {
		//to regularly occurring tasks
		//todo add tasks here that run every second
		if err := pubsub.PushWebhooks(); err != nil {
			log.Println(err) //this needs to be logged and picked up be error managment
		}
	}
}
