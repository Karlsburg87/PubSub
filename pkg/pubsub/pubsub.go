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
		log.Printf("Incorrect Username and Password pair : User UUID %s", rec.UUID)
		return nil, fmt.Errorf("User already exists. Please enter correct credentials to login or select a new username to create a new user.")
	}
	//create user if no username exists
	pubsub.mu.Lock()
	pubsub.Users[user.UsernameHash] = user
	pubsub.mu.Unlock()

	return pubsub.Users[user.UsernameHash], nil
}

//GetTopic gets a topic. If it does not exist it creates a new topic
// using the User as the creator
func (pubsub *PubSub) GetTopic(topicName string, user *User) (topic *Topic, err error) {
	if topic, err = pubsub.FetchTopic(topicName, user); err != nil {
		return pubsub.CreateTopic(topicName, user)
	}
	return topic, nil
}

//FetchTopic fetches a topic or returns an error if not found
func (pubsub *PubSub) FetchTopic(topicName string, user *User) (*Topic, error) {
	if topic, ok := pubsub.Topics[topicName]; ok {
		return topic, nil
	}
	return nil, fmt.Errorf("Topic does not exist")
}

//CreateTopic creates a topic or returns an error if already exists
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

//Tombstone cycles through and does tombstoning and deletion activities
func (pubsub *PubSub) Tombstone(consideredStale, resurrectionOpportunity time.Duration) error {
	//subscription tombstoning
	if err := pubsub.subscriptionTombstone(consideredStale, resurrectionOpportunity); err != nil {
		return err
	}
	//message tombstoning
	if err := pubsub.messageTombstone(resurrectionOpportunity); err != nil {
		return err
	}
	//topic tombstoning
	if err := pubsub.topicTombstone(consideredStale); err != nil {
		return err
	}
	//user tombstoning
	if err := pubsub.userTombstone(resurrectionOpportunity); err != nil {
		return err
	}

	return nil
}

//subscriptionTombstone used in tombstone for running tombstone and delete functions on Subscription objects
func (pubsub *PubSub) subscriptionTombstone(consideredStale, resurrectionOpportunity time.Duration) error {
	for _, topic := range pubsub.Topics {
		//skip topic if has no subscribers
		if len(topic.PointerPositions) < 1 {
			continue
		}
		for pointer, subscribers := range topic.PointerPositions {
			//if pointer is to a message less than `consideredStale` old - leave alone
			if t, err := topic.Messages[pointer].GetCreatedDateTime(); isStale(t, consideredStale) {
				if err != nil {
					return err
				}
				continue
			}
			//else comb through subscriber list and tombstone
			for _, subscriber := range subscribers {
				if subscriber.tombstone != "" {
					tombstoneDate, err := parseTombstoneDateString(subscriber.tombstone)
					if err != nil {
						return err
					}
					if tombstoneDate.Add(resurrectionOpportunity).Before(time.Now()) {
						//delete subscription
						delete(topic.PointerPositions[pointer], subscriber.ID)
						//also delete User Subscriptions list
						delete(pubsub.Users[subscriber.ID].Subscriptions, topic.Name)

					}
					continue
				}
				//tombstone if no previous tombstone
				if err := subscriber.addTombstone(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

//messageTombstone used in tombstone for running tombstone and delete functions on Message objects
//
//Definition of old is zero subscribers at or below messages in this pointer position
func (pubsub *PubSub) messageTombstone(resurrectionOpportunity time.Duration) error {
	//cycle through Topics
	for topicName, topic := range pubsub.Topics {
		//if topic has no messages then skip
		if len(topic.Messages) == 0 {
			continue
		}
		//delete messages from bottom up where subscriber length is 0
		for lowestPosition := (topic.PointerHead - len(topic.PointerPositions)); len(topic.PointerPositions[lowestPosition]) < 1; lowestPosition -= 1 {
			//tombstone if no tombstone already
			if topic.Messages[lowestPosition].tombstone == "" {
				m := pubsub.Topics[topicName].Messages[lowestPosition]
				m.tombstone = tombstoneDateString()
				pubsub.Topics[topicName].Messages[lowestPosition] = m
			}
			//get tombstone
			tombstone, err := parseTombstoneDateString(topic.Messages[lowestPosition].tombstone)
			if err != nil {
				return err
			}
			//check if tombstone is older than resurrectionOpportunity duration
			if isStale(tombstone, resurrectionOpportunity) {
				delete(topic.Messages, lowestPosition)
			}
		}
	}
	return nil
}

//topicTombstone used in tombstone for running tombstone and delete functions on Topic objects
//
//Stale and ready for tombstoning is defined as a Topic with no remaining Messages
//AND older than `consideredStale` length of time
func (pubsub *PubSub) topicTombstone(consideredStale time.Duration) error {
	for topicName, topic := range pubsub.Topics {
		if len(topic.Messages) < 1 {
			//check for existing topicTombstone
			if topic.tombstone == "" {
				//add tombstone if recently eligible
				pubsub.Topics[topicName].tombstone = tombstoneDateString()
				continue
			}
			//else delete if stale
			tdate, err := parseTombstoneDateString(topic.tombstone)
			if err != nil {
				return err
			}
			if isStale(tdate, consideredStale) {
				delete(pubsub.Topics, topicName)
			}
		}
	}
	return nil
}

//userTombstone used in tombstone for running tombstone and delete functions on
//User objects
//
//Definition of stale is a user with no subscriptions and creator of no Topics
func (pubsub *PubSub) userTombstone(resurrectionOpportunity time.Duration) error {
	for usr, user := range pubsub.Users {
		if len(user.Subscriptions) > 0 {
			continue
		}
		//check if they are already tombstoned
		if user.tombstone == "" {
			pubsub.Users[usr].tombstone = tombstoneDateString()
			continue
		}
		//otherwise check if safe to delete
		d, err := parseTombstoneDateString(user.tombstone)
		if err != nil {
			return err
		}
		if isStale(d, resurrectionOpportunity) {
			delete(pubsub.Users, usr)
		}
	}
	return nil
}

//metranome initiates regularly occurring activities
// such as fullfilling push subscriptions, backing up
// and garbage collection of messages
//
//TODO: Errors need to be logged in kv-db and followed up to prevent errors going unchecked or panicking on non catastrophic errors
func (pubsub *PubSub) metranome() {
	//time intervals
	sec := time.Tick(1 * time.Second)
	milliSecs := time.Tick(80 * time.Millisecond)
	min := time.Tick(1 * time.Second * 60)
	//actions for each
	go func() {
		for {
			select {
			case <-min:
				if err := pubsub.Tombstone(3*60*time.Minute, 30*time.Minute); err != nil {
					log.Println(err) //!Needs logging!
				}

			case <-sec:

			case <-milliSecs:
				if err := pubsub.PushWebhooks(); err != nil {
					log.Println(err) //!Needs logging!
				}
			}
		}
	}()
}
