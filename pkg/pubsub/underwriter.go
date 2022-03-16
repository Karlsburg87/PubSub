package pubsub

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

//Underwriter is the default implementation of Persist
//
//It stores all needed persisted files in the ./store directory
type Underwriter struct {
	*PersistCore
	db *bolt.DB
}

//NewUnderwriter creates a new Underwriter instance that implements Persist
func NewUnderwriter(pubsub *PubSub) (*Underwriter, error) {
	if err := os.MkdirAll(path.Join(PersistBase, "messages"), 0766); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path.Join(PersistBase, "underwriter.db"), 0766, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte("user")); err != nil {
			return nil
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("sub")); err != nil {
			return nil
		}
		return nil
	})
	return &Underwriter{
		db: db,
		PersistCore: &PersistCore{
			pubsub:            pubsub,
			userWriter:        make(chan User),
			userDeleter:       make(chan string),
			subscriberWriter:  make(chan PersistSubscriberStruct),
			subscriberDeleter: make(chan PersistSubscriberStruct),
			messageWriter:     make(chan PersistMessageStruct),
			messageDeleter:    make(chan PersistMessageStruct),
		},
	}, nil
}

//Launch spins up all goroutines required to stream writes and deletes
func (uw *Underwriter) Launch() error {
	go func() {
		if err := uw.WriteUser(); err != nil {
			log.Panicln(err)
		}
	}()
	go func() {
		if err := uw.WriteSubscriber(); err != nil {
			log.Panicln(err)
		}
	}()
	go func() {
		if err := uw.WriteMessage(); err != nil {
			log.Panicln(err)
		}
	}()
	go func() {
		if err := uw.DeleteUser(); err != nil {
			log.Panicln(err)
		}
	}()
	go func() {
		if err := uw.DeleteSubscriber(); err != nil {
			log.Panicln(err)
		}
	}()
	go func() {
		if err := uw.DeleteMessage(); err != nil {
			log.Panicln(err)
		}
	}()
	return nil
}

//Switchboard returns the PersistCore field of the underlying Underwriter object
// to make access to channels to goroutines simpler
func (uw *Underwriter) Switchboard() PersistCore {
	return *uw.PersistCore
}

//TidyUp cleans up database connections before close.
// Must run after NewUnderwriter call as defer
// Persist.TidyUp()
func (uw *Underwriter) TidyUp() error {
	uw.db.Close()
	return nil
}

//WriteUser adds a user to the persistence layer
func (uw *Underwriter) WriteUser() error {
	for user := range uw.userWriter {
		//GOB encode user
		var encUser bytes.Buffer
		// Create an encoder and send a value.
		enc := gob.NewEncoder(&encUser)
		err := enc.Encode(user)
		if err != nil {
			return err
		}
		//Save to DB
		if err := uw.db.Batch(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("user"))
			err := b.Put([]byte(user.UsernameHash), encUser.Bytes())
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

//WriteSubscriber adds a subscriber to the persistence layer
func (uw *Underwriter) WriteSubscriber() error {
	for subscriberStruct := range uw.subscriberWriter {
		//GOB encode user
		var encSubscriber bytes.Buffer
		// Create an encoder and send a value.
		enc := gob.NewEncoder(&encSubscriber)
		err := enc.Encode(subscriberStruct.Subscriber)
		if err != nil {
			return err
		}
		//Save to DB
		if err := uw.db.Batch(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("sub"))
			err := b.Put([]byte(fmt.Sprintf("%s/%d/%s", subscriberStruct.TopicName, subscriberStruct.MessageID, subscriberStruct.Subscriber.ID)), encSubscriber.Bytes())
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

//WriteMessage adds a message to the persistence layer
func (uw *Underwriter) WriteMessage() error {
	for messageStruct := range uw.messageWriter {
		//store as file in `/store` directory
		dirStructure := path.Join(PersistBase, "messages", messageStruct.TopicName)
		loc := path.Join(dirStructure, fmt.Sprintf("/%d.json", messageStruct.Message.ID))

		if err := os.MkdirAll(dirStructure, 0766); err != nil {
			return fmt.Errorf("error creating dir structure in Underwriter.WriteMessage: %v", err)
		}
		file, err := os.Create(loc)
		if err != nil {
			return fmt.Errorf("error creating message file (%s) in Underwriter.WriteMessage: %v", loc, err)
		}
		//write Json data to file - so human readable
		enc := json.NewEncoder(file)
		if err := enc.Encode(messageStruct.Message); err != nil {
			return fmt.Errorf("error encoding Message in Underwriter.WriteMessage: %v", err)
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

//GetUseret returns a single user by userID string
// (Which is also UsernameHash of the user)
func (uw *Underwriter) GetUser(userID string) (User, error) {
	var decUser User
	//get User
	if err := uw.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("user"))
		encUser := b.Get([]byte(userID))
		//GOB decode user
		dec := gob.NewDecoder(bytes.NewReader(encUser))
		if err := dec.Decode(&decUser); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return User{}, err
	}
	//update pointers
	decUser.persistLayer = uw.pubsub.persistLayer

	return decUser, nil
}

//GetSubscriberet returns a single subscriber by
// subcriberID (which is also the userID attachted to
// the subscriber),messageID and topicName
func (uw *Underwriter) GetSubscriber(subscriberID string, messageID int, topicName string) (Subscriber, error) {
	var decSubscriber Subscriber
	//get User
	if err := uw.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("sub"))
		encSub := b.Get([]byte(fmt.Sprintf("%s/%d/%s", topicName, messageID, subscriberID)))
		//GOB decode user
		dec := gob.NewDecoder(bytes.NewReader(encSub))
		if err := dec.Decode(&decSubscriber); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return Subscriber{}, err
	}
	return decSubscriber, nil
}

//GetMessage returns a single message by messageID and topicName
func (uw *Underwriter) GetMessage(messageID int, topicName string) (Message, error) {
	file, err := os.Open(path.Join(PersistBase, fmt.Sprintf("%s/%d.json", topicName, messageID)))
	if err != nil {
		return Message{}, err
	}
	defer file.Close()
	var message Message
	dec := json.NewDecoder(file)
	if err := dec.Decode(&message); err != nil {
		return Message{}, err
	}
	return message, nil
}

//StreamUsers returns a chan through which it streams
// all Users from the db
func (uw *Underwriter) StreamUsers() (chan Streamer, error) {
	return uw.streamBucket(PersistUser)
}

//StreamSubscribers returns a chan through which it streams all
// Subscribers from the db
func (uw *Underwriter) StreamSubscribers() (chan Streamer, error) {
	return uw.streamBucket(PersistSubscriber)
}

//StreamMessages returns a chan through which it streams all
// Messages from the db
func (uw *Underwriter) StreamMessages() (chan Streamer, error) {
	streamer := make(chan Streamer)
	go func() {
		messageStreamer(path.Join(PersistBase, "messages"), streamer)
		close(streamer)
		streamer = nil
	}()

	return streamer, nil
}

//DeleteUser accepts UserID which is the userhash string
func (uw *Underwriter) DeleteUser() error {
	for usrID := range uw.userDeleter {
		if err := uw.db.Batch(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("user"))
			err := b.Delete([]byte(usrID))
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

//DeleteSubscriber accepts subscriberID (the userID of
// the subscription), messageID and topicName
//
//Add messageID as -1 if not available. Func will they cycle through the topic and delete matches to subscriberID
func (uw *Underwriter) DeleteSubscriber() error {
	for subsc := range uw.subscriberDeleter {
		if err := uw.db.Batch(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("sub"))
			if subsc.MessageID >= 0 {
				if err := b.Delete([]byte(fmt.Sprintf("%s/%d/%s", subsc.TopicName, subsc.MessageID, subsc.SubscriberID))); err != nil {
					return fmt.Errorf("error issuing Subscriber Delete in BoltDB:%v", err)
				}
				return nil
			}
			c := b.Cursor()

			//cycle through all messages in topic and delete matching subscriberID
			prefix := []byte(fmt.Sprintf("%s/", subsc.TopicName))
			for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
				//check suffix
				if strings.HasSuffix(string(k), subsc.SubscriberID) {
					if err := b.Delete(k); err != nil {
						return fmt.Errorf("error doing delete of prefix/suffix match in Subscriber Delete in BoltDB :%v", err)
					}
				}

			}

			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

//DeleteMessage accepts messageID and topicName
func (uw *Underwriter) DeleteMessage() error {
	for msg := range uw.messageDeleter {
		if err := os.Remove(path.Join(PersistBase, fmt.Sprintf("messages/%s/%d.json", msg.TopicName, msg.MessageID))); err != nil {
			return err
		}
	}
	return nil
}

//-----------------------------------Helpers

// messageStreamer is the recursive function used to walk messages in blob storage and stream them to the application
func messageStreamer(basePath string, streamer chan Streamer) {
	files, err := os.ReadDir(basePath)
	if err != nil { //Better error handling required
		log.Printf("%v", fmt.Errorf("error reading directory: %v", err))
		return
	}
	for _, file := range files {
		//recursive call if dir
		if file.IsDir() {
			messageStreamer(path.Join(basePath, file.Name()), streamer)
			continue
		}
		f, err := os.Open(path.Join(basePath, file.Name()))
		if err != nil { //Better error handling required
			log.Printf("%v", fmt.Errorf("error reading directory in messageStreamer: %v", err))
			return
		}
		//pull content
		content, err := io.ReadAll(f)
		if err != nil { //Better error handling required
			log.Printf("%v", fmt.Errorf("error reading directory in messageStreamer: %v", err))
			return
		}
		//unmarshal from JSON
		m := &Message{}
		if err := json.Unmarshal(content, m); err != nil {
			log.Printf("%v", fmt.Errorf("error unmarshaling JSON in messageStreamer: %v", err))
			return
		}
		//Stream out
		streamer <- Streamer{
			Key:  path.Join(basePath, file.Name()),
			Unit: m,
		}
		if err := f.Close(); err != nil {
			log.Printf("%v", fmt.Errorf("error closing file in messageStreamer: %v", err))
			return
		}
	}
}

//streamBucket is the user and subscriber object generic boltDB bucket streaming function
func (uw Underwriter) streamBucket(streamType PersistUnit) (chan Streamer, error) {
	streamer := make(chan Streamer)
	bucketName := ""
	s := Streamer{}
	switch streamType {
	case PersistUser:
		bucketName = "user"
		s.Unit = &User{}
	case PersistSubscriber:
		bucketName = "sub"
		s.Unit = &Subscriber{}
	default:
		return nil, fmt.Errorf("streamType must be either PersistUser or PersistSubscriber")
	}
	go func(bucketName string) {
		if err := uw.db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(bucketName))
			c := b.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				dec := gob.NewDecoder(bytes.NewReader(v))
				switch s.Unit.(type) {
				case *Subscriber:
					subscr := &Subscriber{}
					if err := dec.Decode(subscr); err != nil {
						return err
					}
					s.Unit = subscr
				case *User:
					usr := &User{}
					if err := dec.Decode(usr); err != nil {
						return err
					}
					s.Unit = usr
				}
				s.Key = string(k)
				streamer <- s
			}

			return nil
		}); err != nil {
			log.Println(err)
		}
		close(streamer)
		streamer = nil
	}(bucketName)
	return streamer, nil
}
