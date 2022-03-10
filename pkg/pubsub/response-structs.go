package pubsub

import "encoding/json"

//ListKeysResp is the response from a query requesting lists of entries such as /users/fetch, /topics/fetch, etc
type ListKeysResp struct {
	Error       string   `json:"error,omitempty"`
	Topics      []string `json:"topics,omitempty"`
	Users       []string `json:"users,omitempty"` //not in use yet
	Subscribers []string `json:"subs,omitempty"`  //not in use yet
	Count       int      `json:"count"`
}

//CreateUserResp is the response from a create user request
type CreateUserResp struct {
	Error string `json:"error,omitempty"`
	UUID  string `json:"user_id,omitempty"`
	//SubscriptionCount is len(User.Subscriptions)
	SubscriptionCount int               `json:"subscription_count"`
	Subscriptions     map[string]string `json:"subscriptions,omitempty"`
	Created           string            `json:"created,omitempty"`
}

//MessageResp is the response from Message orientated requests
type MessageResp struct {
	Error   string  `json:"error,omitempty"`
	Topic   string  `json:"topic_id,omitempty"`
	Message Message `json:"message,omitempty"`
}

//TopicResp is the response form for Topic orientated requests
type TopicResp struct {
	Error       string `json:"error,omitempty"`
	Topic       string `json:"topic_name"`
	Status      string `json:"status"`
	Creator     string `json:"creator"`
	PointerHead int    `json:"pointer_head"`
	//CanWrite shows if requester User can write to the topic (userID
	// matches topic.Creator.ID)
	CanWrite bool `json:"writable"`
}

//SubscribeResp is the response form for Subscription orientated requests
type SubscribeResp struct {
	Error  string `json:"error,omitempty"`
	User   string `json:"user_id"`
	Topic  string `json:"topic_name"`
	Status string `json:"status"`
	//CanWrite shows if the requester User can write to the topic (userID
	// matches topic.Creator.ID)
	CanWrite bool `json:"writable"`
}

//------------------------------------------- Request Struct

//IncomingReq is the standard structure for message requests to the service
type IncomingReq struct {
	Username string `json:"username"` //Mandatory
	Password string `json:"password"` //Mandatory
	Topic    string `json:"topic,omitempty"`
	//WebhookURL for push subscription to topic
	WebhookURL string `json:"webhook_url,omitempty"`
	//Message used for writing messages to services
	Message interface{} `json:"message,omitempty"`
	//MessageID used for pulling messages from topics
	MessageID int `json:"message_id,omitempty"`
}

//------------------------------------------- interface

//Responder are handler response objects with encoding methods
type Responder interface {
	toJSON() ([]byte, error)
}

//toJSON marshalls the response object to JSON binary
func (response ListKeysResp) toJSON() ([]byte, error) {
	return json.MarshalIndent(response, " ", " ")
}

//toJSON marshalls the response object to JSON binary
func (response CreateUserResp) toJSON() ([]byte, error) {
	return json.MarshalIndent(response, " ", " ")
}

//toJSON marshalls the response object to JSON binary
func (response MessageResp) toJSON() ([]byte, error) {
	return json.MarshalIndent(response, " ", " ")
}

//toJSON marshalls the response object to JSON binary
func (response TopicResp) toJSON() ([]byte, error) {
	return json.MarshalIndent(response, " ", " ")
}

//toJSON marshalls the response object to JSON binary
func (response SubscribeResp) toJSON() ([]byte, error) {
	return json.MarshalIndent(response, " ", " ")
}
