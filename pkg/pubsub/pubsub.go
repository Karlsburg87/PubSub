package pubsub

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

//Close is the tidy-up script that should be used as a defer after calling getReady function
func (pubsub *PubSub) Close() error {
	if err := pubsub.persistLayer.TidyUp(); err != nil {
		return err
	}
	return nil
}

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
	//Add access to persistLayer
	user.persistLayer = pubsub.persistLayer
	pubsub.mu.RLock()
	if rec, ok := pubsub.Users[user.UsernameHash]; ok { //username found so check password...
		if user.PasswordHash == rec.PasswordHash { //password correct so return User
			pubsub.mu.RUnlock()
			return rec, nil
		}
		//password incorrect return error
		pubsub.mu.RUnlock()
		return nil, fmt.Errorf("user already exists - please enter correct credentials to login or select a new username to create a new user")
	}
	pubsub.mu.RUnlock()
	//share access to the core persistLayer with new user - no lock as init once at startup
	user.persistLayer = pubsub.persistLayer
	//add user if no username exists
	pubsub.mu.Lock()
	pubsub.Users[user.UsernameHash] = user
	pubsub.mu.Unlock()
	//Persist the newly created user
	pubsub.persistLayer.Switchboard().userWriter <- *user

	return user, nil
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
	pubsub.mu.RLock()
	defer pubsub.mu.RUnlock()
	if topic, ok := pubsub.Topics[topicName]; ok {
		return topic, nil
	}
	return nil, fmt.Errorf("Topic does not exist")
}

//CreateTopic creates a topic or returns an error if already exists
func (pubsub *PubSub) CreateTopic(topicName string, user *User) (*Topic, error) {
	//Return error if the topic already exists
	pubsub.mu.RLock()
	if _, ok := pubsub.Topics[topicName]; ok {
		pubsub.mu.RUnlock()
		return nil, fmt.Errorf("Topic already exists")
	}
	pubsub.mu.RUnlock()

	newTopic := &Topic{
		Creator:          user.UUID,
		Name:             topicName,
		PointerHead:      0,
		PointerPositions: make(map[int]Subscribers),
		Messages:         make(map[int]Message),
		mu:               &sync.RWMutex{},
		sseOut:           pubsub.sseDistro.Intake,
	}
	//Add the topic to the public topic list
	pubsub.mu.Lock()
	defer pubsub.mu.Unlock()
	pubsub.Topics[newTopic.Name] = newTopic
	//subscribe the User
	p := pubsub.Topics[topicName]
	user.Subscribe(p, "")
	//remove any tombstones on the user
	if err := user.removeTombstone(); err != nil {
		return nil, err
	}

	return p, nil
}

//PushWebhooks runs through all topics and pushes messages to
// the subscribers as a Webhook service. Exponential backoff for non 201/200 unacknoledged pushes up to 1 hour attempt intervals.
//
//Should run through continuously
func (pubsub *PubSub) PushWebhooks() error {
	//cycle through Topics
	for _, topic := range pubsub.Topics {
		//cycle through Messages
		for msgID, message := range topic.Messages {
			//cycle must complete before exit
			wg := &sync.WaitGroup{}
			//send message to all push subscribers
			topic.mu.RLock()
			for _, subscriber := range topic.PointerPositions[msgID] {
				wg.Add(1)
				go pubsub.webhookRoutine(topic, message, subscriber, wg)
			}
			topic.mu.RUnlock()
			wg.Wait()
		}
	}
	return nil
}

//webhookRoutine is the goroutine does the push via http.POST with built in exponential backoff from the default push cycle. Intended for use in PushWebhooks
func (pubsub *PubSub) webhookRoutine(topic *Topic, message Message, subscriber *Subscriber, wg *sync.WaitGroup) {
	defer wg.Done()
	if subscriber.PushURL != "" {
		//exit it still need to backoff from last send
		if !subscriber.lastpushAttempt.IsZero() && subscriber.lastpushAttempt.Add(subscriber.backoff).After(time.Now()) {
			return
		}
		//push to url and await for 200/201 acknolegement
		msgParcel := MessageResp{
			Topic:   topic.Name,
			Message: message,
		}
		parcel, err := msgParcel.toJSON()
		if err != nil { //This needs logging for followup as not dealt with here
			log.Printf("Error converting to JSON from webhookRoutine goroutine: %v", err)
			return
		} //?echo err and continue?
		resp, err := http.Post(subscriber.PushURL, "application/json", bytes.NewReader(parcel))
		if err != nil || (resp.StatusCode != 200 && resp.StatusCode != 201) {
			//debug logging
			log.Println(fmt.Errorf("could not deliver msg: error: %v (StatusCode: %d)\nSubscriber: %s, [Message: %+v]", err, resp.StatusCode, subscriber.ID, msgParcel))

			pubsub.mu.RLock()
			p := pubsub.Topics[topic.Name]
			pubsub.mu.RUnlock()

			p.mu.Lock()
			//set backoff for next attempt
			subscriber.lastpushAttempt = time.Now()
			if subscriber.backoff == 0 {
				subscriber.backoff = 80 * time.Millisecond
			}
			subscriber.backoff = subscriber.backoff * 2
			//cap exponential backoff at 1 hour
			if subscriber.backoff > (60 * time.Minute) {
				subscriber.backoff = 60 * time.Minute
			}
			p.mu.Unlock()

			return
		}
		//push subscriber pointer up an index place
		pubsub.mu.RLock()
		p := pubsub.Topics[topic.Name]
		pubsub.mu.RUnlock()

		p.mu.Lock()
		//reset the backoff fields
		subscriber.lastpushAttempt = time.Time{}
		subscriber.backoff = 80 * time.Millisecond
		//move up to next pointer position
		if _, ok := pubsub.Topics[topic.Name].PointerPositions[message.ID+1]; !ok {
			pubsub.Topics[topic.Name].PointerPositions[message.ID+1] = make(Subscribers)
		}
		pubsub.Topics[topic.Name].PointerPositions[message.ID+1][subscriber.ID] = subscriber
		//delete previous pointer position record
		delete(pubsub.Topics[topic.Name].PointerPositions[message.ID], subscriber.ID)
		p.mu.Unlock()
	}
}

//Tombstone cycles through and does tombstoning and deletion activities
//
//ConsideredStale is the time duration after which an item is considered stale and okay to tombstone
//
//resurrectionOpportunity is the time duration after which a tombstoned item can be deleted. This leaves an opportunity between tombstoning and deletion to be saved (by becoming active again)
//
//
//N.B. This function blocks all PubSub activity with a PubSub Lock - so should be run conservatively and opportunistically
func (pubsub *PubSub) Tombstone(consideredStale, resurrectionOpportunity time.Duration) error {
	//This function blocks all PubSub execution so should be run conservatively and opportunistically
	log.Println("Tombstone Procedure KO")
	pubsub.mu.Lock()
	defer pubsub.mu.Unlock()
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
		//skip topic if has no subscribers or if topic has no messages
		if len(topic.PointerPositions) < 1 || len(topic.Messages) < 1 {
			continue
		}
		for pointer, subscribers := range topic.PointerPositions {
			//skip pointer to head+1 message as these are queued awaiting the next added message
			if pointer > topic.PointerHead {
				continue
			}
			//if pointer is to a message less than `consideredStale` old - leave alone
			if t, err := topic.Messages[pointer].GetCreatedDateTime(); !isStale(t, consideredStale) {
				if err != nil {
					//send debug info to std.out
					log.Printf("{Error: \"GetCreatedDateTime from subscriptionTombstone\", Subscriber: %+v, Topic: \"%s\", Pointer: %d, Message Count: %d, Message: %+v}\n", topic.PointerPositions[pointer], topic.Name, pointer, len(topic.Messages), topic.Messages[pointer])

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
						if _, ok := pubsub.Users[subscriber.UsernameHash]; ok {
							delete(pubsub.Users[subscriber.UsernameHash].Subscriptions, topic.Name) //need User.UsernameHash here instead of User.UUID
						} else {
							log.Printf("Should be able to find user %s to delete topic.Name from Subscriptions but can not.", subscriber.UsernameHash)
						}
						//delete from persist store
						pubsub.persistLayer.Switchboard().subscriberDeleter <- PersistSubscriberStruct{
							MessageID:    pointer,
							TopicName:    topic.Name,
							SubscriberID: subscriber.ID,
						}
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
		for lowestPosition := (topic.PointerHead - len(topic.PointerPositions)); len(topic.PointerPositions[lowestPosition]) < 1 && topic.PointerPositions[lowestPosition] != nil; lowestPosition += 1 {
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
				//remove from persist store
				pubsub.persistLayer.Switchboard().messageDeleter <- PersistMessageStruct{
					TopicName: topicName,
					MessageID: lowestPosition,
				}
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
			//delete from persist store
			pubsub.persistLayer.Switchboard().userDeleter <- usr
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
	sec := time.NewTicker(1 * time.Second)
	milliSecs := time.NewTicker(80 * time.Millisecond)
	min := time.NewTicker(1 * time.Second * 60)
	//actions for each
	go func() {
		for {
			select {
			case <-min.C:
				if err := pubsub.Tombstone(durationToStale, durationForResurrect); err != nil {
					log.Println(err) //!Needs logging!
				}

			case <-sec.C:

			case <-milliSecs.C:
				if err := pubsub.PushWebhooks(); err != nil {
					log.Println(err) //!Needs logging!
				}
			}
		}
	}()
}
