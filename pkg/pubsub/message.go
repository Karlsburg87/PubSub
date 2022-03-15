package pubsub

import (
	"fmt"
	"time"
)

//GetCreatedDateTime fetches the created datetime string and parses it
func (message Message) GetCreatedDateTime() (time.Time, error) {
	if message.Created == "" {
		return time.Time{}, fmt.Errorf("Error as no date string exists.\nMessage: %+v", message)
	}
	return time.Parse(time.RFC3339, message.Created)
}

//AddCreatedDatestring adds the given time to the message Created field as a formatted string
func (message *Message) AddCreatedDatestring(time.Time) string {
	message.Created = time.Now().Format(time.RFC3339)
	return message.Created
}
