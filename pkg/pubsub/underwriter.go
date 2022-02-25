package pubsub

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	bolt "go.etcd.io/bbolt"
	"io"
	"log"
	"os"
	"path"
	"time"
)

//Underwriter is the default implementation of Persist
//
//It stores all needed persisted files in the ./store directory
type Underwriter struct {
	pubsub *PubSub
	db     *bolt.DB
}

func NewUnderwriter(pubsub *PubSub) (*Underwriter, error) {
	db, err := bolt.Open(path.Join(PersistBase, "underwriter.db"), 0666, &bolt.Options{Timeout: 1 * time.Second})
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
		pubsub: pubsub,
		db:     db,
	}, nil
}

//TidyUp cleans up database connections before close.
// Must run after NewUnderwriter call as defer
// Persist.TidyUp()
func (uw *Underwriter) TidyUp() error {
	uw.db.Close()
	return nil
}

//WriteUser adds a user to the persistance layer
func (uw Underwriter) WriteUser(user *User) error {
	//GOB encode user
	var encUser bytes.Buffer
	// Create an encoder and send a value.
	enc := gob.NewEncoder(&encUser)
	err := enc.Encode(user)
	if err != nil {
		return err
	}
	//Save to DB
	return uw.db.Batch(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("user"))
		err := b.Put([]byte(user.UsernameHash), encUser.Bytes())
		return err
		return nil
	})
}

//WriteSubscriber adds a subscriber to the persistance layer
func (uw Underwriter) WriteSubscriber(subscriber *Subscriber, message Message, topic *Topic) error {
	//GOB encode user
	var encSubscriber bytes.Buffer
	// Create an encoder and send a value.
	enc := gob.NewEncoder(&encSubscriber)
	err := enc.Encode(subscriber)
	if err != nil {
		return err
	}
	//Save to DB
	return uw.db.Batch(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("sub"))
		err := b.Put([]byte(fmt.Sprintf("%s/%d/%s", topic.Name, message.ID, subscriber.ID)), encSubscriber.Bytes())
		return err
		return nil
	})
}

//WriteMessage adds a message to the persistance layer
func (uw Underwriter) WriteMessage(message Message, topic *Topic) error {
	//store as file in `/store` directory
	loc := path.Join(PersistBase, fmt.Sprintf("%s/%s.json", topic.Name, message.ID))
	if err := os.MkdirAll(loc, 0766); err != nil {
		return err
	}
	file, err := os.Create(path.Join(loc, fmt.Sprintf("%s.json", message.ID)))
	if err != nil {
		return err
	}
	defer file.Close()
	//write Json data to file - so human readable
	enc := json.NewEncoder(file)
	if err := enc.Encode(message); err != nil {
		return err
	}

	return nil
}

//GetUseret returns a single user by userID string
// (Which is also UsernameHash of the user)
func (uw Underwriter) GetUser(userID string) (User, error) {
	var decUser User
	//get User
	if err := uw.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("user"))
		encUser := b.Get([]byte(userID))
		//GOB decode user
		dec := gob.NewDecoder(bytes.NewReader(encUser))
		if err := dec.Decode(decUser); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return User{}, err
	}
	return decUser, nil
}

//GetSubscriberet returns a single subscriber by
// subcriberID (which is also the userID attachted to
// the subscriber),messageID and topicName
func (uw Underwriter) GetSubscriber(subscriberID string, messageID int, topicName string) (Subscriber, error) {
	var decSubscriber Subscriber
	//get User
	if err := uw.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("sub"))
		encUser := b.Get([]byte(fmt.Sprintf("%s/%d/%s", topicName, messageID, subscriberID)))
		//GOB decode user
		dec := gob.NewDecoder(bytes.NewReader(encUser))
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
func (uw Underwriter) GetMessage(messageID int, topicName string) (Message, error) {
	file, err := os.Open(path.Join(PersistBase, fmt.Sprintf("%s/%s.json", topicName, messageID)))
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
func (uw Underwriter) StreamUsers() (chan Streamer, error) {
	return uw.streamBucket(PersistUser)
}

//StreamSubscribers returns a chan through which it streams all
// Subscribers from the db
func (uw Underwriter) StreamSubscribers() (chan Streamer, error) {
	return uw.streamBucket(PersistSubscriber)
}

//StreamMessages returns a chan through which it streams all
// Messages from the db
func (uw Underwriter) StreamMessages() (chan Streamer, error) {
	streamer := make(chan Streamer)
	go messageStreamer(PersistBase, streamer)

	return streamer, nil
}

//DeleteUser accepts UserID which is the userhash string
func (uw Underwriter) DeleteUser(userID string) error {
	return uw.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("user"))
		err := b.Delete([]byte(userID))
		return err
	})
}

//DeleteSubscriber accepts subscriberID (the userID of
// the subscription), messageID and topicName
func (uw Underwriter) DeleteSubscriber(topicName string, messageID int, subscriberID string) error {
	return uw.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("sub"))
		err := b.Delete([]byte(fmt.Sprintf("%s/%d/%s", topicName, messageID, subscriberID)))
		return err
	})
}

//DeleteMessage accepts messageID and topicName
func (uw Underwriter) DeleteMessage(messageID int, topicName string) error {
	return os.Remove(path.Join(PersistBase, fmt.Sprintf("%s/%s.json", topicName, messageID)))
}

//-----------------------------------Helpers

// messageStreamer is the recursive function used to walk messages in blob storage and stream them to the application
func messageStreamer(basePath string, streamer chan Streamer) {
	files, err := os.ReadDir(basePath)
	if err != nil { //Better error handling required
		log.Printf("%v", fmt.Errorf("Error reading directory"))
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
			log.Printf("%v", fmt.Errorf("Error reading directory"))
			return
		}
		//pull content
		content, err := io.ReadAll(f)
		if err != nil { //Better error handling required
			log.Printf("%v", fmt.Errorf("Error reading directory"))
			return
		}
		//unmarshal from JSON
		m := &Message{}
		if err := json.Unmarshal(content, &m); err != nil {
			log.Printf("%v", fmt.Errorf("Error unmarshaling JSON"))
			return
		}
		//Stream out
		streamer <- Streamer{
			Key:  file.Name(),
			Unit: m,
		}
		if err := f.Close(); err != nil {
			log.Printf("%v", fmt.Errorf("Error closing file"))
			return
		}
	}
}

//generic boltDB bucket streaming
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
	go func() {
		if err := uw.db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(bucketName))
			c := b.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				dec := gob.NewDecoder(bytes.NewReader(v))
				if err := dec.Decode(&s.Unit); err != nil {
					return err
				}
				s.Key = string(k)
				streamer <- s
			}
			return nil
		}); err != nil {
			log.Println(err)
		}
	}()
	return streamer, nil
}
