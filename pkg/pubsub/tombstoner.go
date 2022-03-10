package pubsub

import "time"

/**
* The execution of the tombstone interface occures at regular intevals executed in
* pubsub.metranome. The full logic of running the deletions is in the pubsub.go file.
**/

//----------------tombstone interface

//tombstoner standardises the method of deleting User,
// Topic, Subscriber and Message due to non action and orphaning
//
//The deletion task is done by the pubsub.bury task cron
type tombstoner interface {
	//addTombstone is for tagging inactive or orphaned entities. Tombstone timestamp tagging readies for bury after fixed duration if no activity found or links created
	addTombstone() error
	//removeTombstone removes tombstone tags from previously tombstoned entities that have since shown activity
	removeTombstone() error
}

//-------------------Implementations

//addTombstone exists to implement tombstoner
func (user *User) addTombstone() error {
	user.tombstone = tombstoneDateString()
	return nil
}

//removeTombstone exists to implement tombstoner
func (user *User) removeTombstone() error {
	user.tombstone = ""
	return nil
}

//addTombstone exists to implement tombstoner
func (subscriber *Subscriber) addTombstone() error {
	subscriber.tombstone = tombstoneDateString()
	return nil
}

//removeTombstone exists to implement tombstoner
func (subscriber *Subscriber) removeTombstone() error {
	subscriber.tombstone = ""
	return nil
}

//addTombstone exists to implement tombstoner
func (topic *Topic) addTombstone() error {
	topic.tombstone = tombstoneDateString()
	return nil
}

//removeTombstone exists to implement tombstoner
func (topic *Topic) removeTombstone() error {
	topic.tombstone = ""
	return nil
}

//addTombstone exists to implement tombstoner
func (message *Message) addTombstone() error {
	message.tombstone = tombstoneDateString()
	return nil
}

//removeTombstone exists to implement tombstoner
func (message *Message) removeTombstone() error {
	message.tombstone = ""
	return nil
}

//-------------------Helper functions

//tombstoneDateString creates a formatted RFC3339 date
// for the tombstone field
func tombstoneDateString() string {
	return time.Now().Format(time.RFC3339)
}

//parseTombstoneDateString returns the time.Time
// represented by the tombstone field
func parseTombstoneDateString(tombstoneDate string) (time.Time, error) {
	return time.Parse(time.RFC3339, tombstoneDate)
}
