package pubsub

import (
	"fmt"
	"net/url"
	"sync"
	"time"
)

//Subscribe method subscribes the user to the given topic using
// the given pushURL. If no pushURL, subscription is pull type
// using the topic ID.
func (user *User) Subscribe(topic *Topic, pushURL string) error {
	//check pushURL is valid
	var url *url.URL
	var err error
	if pushURL != "" {
		url, err = url.Parse(pushURL)
		if err != nil {
			return fmt.Errorf("push URL not valid: %v", err)
		}
	}
	//Create Subsriber Object
	sub := &Subscriber{
		ID:      user.UUID,
		PushURL: url,
		mu:      &sync.RWMutex{},
		backoff: 80 * time.Millisecond,
		Creator: topic.Creator == user.UUID,
	}

	//unsubscribe from topic first if already a subscriber.
	//This will ensure there are no multiple subscriptions in // various pointer positions. Will also give consistent
	// expected performance for Subscription to be at the
	// head position from the last point at which it was called
	if err := user.Unsubscribe(topic); err != nil {
		return fmt.Errorf("Error when unsubscribing before resubscribing: %v", err)
	}

	user.mu.Lock()
	//add to User subscriber list
	user.Subscriptions[topic.Name] = pushURL
	//remove any user tombstones
	if err := user.removeTombstone(); err != nil {
		user.mu.Unlock()
		return err
	}
	user.mu.Unlock()

	topic.mu.Lock()
	//Add subscriber object to topic to receive messages from
	// current head position
	if _, ok := topic.PointerPositions[topic.PointerHead]; !ok {
		topic.PointerPositions[topic.PointerHead] = make(Subscribers)
	}
	topic.PointerPositions[topic.PointerHead][sub.ID] = sub
	//remove any topic tombstones
	if err := topic.removeTombstone(); err != nil {
		topic.mu.Unlock()
		return err
	}
	topic.mu.Unlock()

	//persist the Subscriptions
	user.persistLayer.Switchboard().subscriberWriter <- PersistSubscriberStruct{
		Subscriber: *sub,
		MessageID:  topic.PointerHead,
		TopicName:  topic.Name,
	}

	return nil
}

//Unsubscribe helper function to unsubscribe a user from a topic
func (user *User) Unsubscribe(topic *Topic) error {
	user.mu.Lock()
	//remove from User subscriber list
	delete(user.Subscriptions, topic.Name)
	//remove from topic pointerPosition list to no
	// longer receive messages
	//remove any user tombstones
	if err := user.removeTombstone(); err != nil {
		user.mu.Unlock()
		return err
	}
	user.mu.Unlock()

	topic.mu.Lock()
	//may have subscription loc other than head position or subscribed more than once
	for pos := range topic.PointerPositions {
		delete(topic.PointerPositions[pos], user.UUID)
	}
	topic.mu.Unlock()

	//delete from persist layer
	user.persistLayer.Switchboard().subscriberDeleter <- PersistSubscriberStruct{
		TopicName:    topic.Name,
		MessageID:    -1,
		SubscriberID: user.UUID,
	}

	return nil
}

//WriteToTopic manages the user writing to a topic it is a creator of
func (user *User) WriteToTopic(topic *Topic, message Message) (Message, error) {
	//check user is the creator of the topic
	if user.UUID != topic.Creator {
		return Message{}, fmt.Errorf("User does not have the authorisation to write to this channel")
	}
	topic.mu.Lock()
	//Add message to topic's message queue
	message.ID = topic.PointerHead
	topic.Messages[topic.PointerHead] = message
	topic.PointerHead += 1
	//remove any topic tombstones
	if err := topic.removeTombstone(); err != nil {
		topic.mu.Unlock()
		return Message{}, err
	}
	topic.mu.Unlock()

	//move the creator's auto subscription up to the PinterHead with no tombstones
	user.Subscribe(topic, "") //removes any existing subscriptions

	user.mu.Lock()
	if err := user.removeTombstone(); err != nil {
		user.mu.Unlock()
		return Message{}, err
	}
	user.mu.Unlock()

	//Persist message
	user.persistLayer.Switchboard().messageWriter <- PersistMessageStruct{
		Message:   message,
		TopicName: topic.Name,
	}

	return message, nil
}

//PullMessage retrieves a message from the Topic message queue if the user is subscibed
func (user *User) PullMessage(topic *Topic, messageID int) (Message, error) {
	//check user is subscribed and isn't pulling a push sub
	user.mu.RLock()
	pushURL, ok := user.Subscriptions[topic.Name]
	user.mu.RUnlock()
	if !ok {
		return Message{}, fmt.Errorf("User not subscribed to Topic")
	} else if pushURL != "" {
		return Message{}, fmt.Errorf("Not Allowed. User attempting to pull from push subscription")
	}
	//get message from position if exists
	topic.mu.Lock()
	defer topic.mu.Unlock()
	if msg, ok := topic.Messages[messageID]; ok {
		//Move pointer
		for pos, subs := range topic.PointerPositions {
			if s, ok := subs[user.UUID]; ok {
				if messageID > pos {
					break
				}
				topic.PointerPositions[messageID][user.UUID] = s
				delete(topic.PointerPositions[pos], user.UUID)

				user.mu.Lock()
				//remove any user tombstones
				if err := user.removeTombstone(); err != nil {
					return Message{}, err
				}
				user.mu.Unlock()
			}
		}
		return msg, nil
	}
	return Message{}, fmt.Errorf("This message does not exist. Head point is %d", topic.PointerHead)
}

//------------------helpers

//GetCreatedDateTime fetches the created datetime string and parses it
func (user User) GetCreatedDateTime() (time.Time, error) {
	if user.Created == "" {
		return time.Time{}, fmt.Errorf("No date string exists.\nUser: %+v\n", user)
	}
	return time.Parse(time.RFC3339, user.Created)
}

//AddCreatedDatestring adds the given time to the message Created field as a formatted string
func (user *User) AddCreatedDatestring(time.Time) string {
	user.Created = time.Now().Format(time.RFC3339)
	return user.Created
}
