[![Go Report Card](https://goreportcard.com/badge/github.com/CDennis-CR/PubSub)](https://goreportcard.com/report/github.com/CDennis-CR/PubSub)
[![Run on Repl.it](https://repl.it/badge/github/CDennis-CR/PubSub)](https://repl.it/github/CDennis-CR/PubSub) 

# PubSub
An RESTful HTTP service to make signing up for event streams easy and open to anyone able to access the URI endpoint. No access permissions barriers or web UI required. Useful for passing in things like uptime stats.

> Pubsub guarentees '*at least once*' message delilvery.

Acknowledgement based system to ensure message delivery guarentees are met.
- Push Subscriptions (Webhooks) need return a 200 or 201 status code to acknowlege. 
- Message pull subscriptions acknowlege message receipt of earlier pointer positions when requesting a later pointer position.

## Status
**pubSub** is in initial development (v0) stage and subject to constant change to its API.

**Not yet suitable for deployment in production environments**

## Data storage
Pubsub is an in-memory system, however it will persist message data to a file-based KV database in future to assist with disaster recovery efforts. See the *Roadmap* section

## Usage
### Request Params
Parameters can be sent by URL query, HTTP Post  JSON payload or a mixture of both.

example URL encoded:
```http
https://some.endpoint/users/create?username=usrname&password=pswrd
```

JSON format for post requests with all params: 
```JSON
{
  "username"    : "username",
  "password"    : "password",
  "topic"       : "topic name",
  "webhook_url" : "https://webhook-url.dev",
  "message"     : "A text message which could be anything - XML, JSON, markdown, etc",
  "message_id"  : 0,
}
```
Go Struct representation:
```go
//IncomingReq is the standard structure for message requests to the service
type IncomingReq struct{
  //Username is a mandatory field
  Username    string       `json:"username"`
  //Password is a mandatory field
  Password    string       `json:"password"`
  //Topic is the `name` of the topic requested 
  Topic       string       `json:"topic,omitempty"`
  //WebhookURL (aka PushURL) for push subscription to topic
  WebhookURL  string       `json:"webhook_url,omitempty"`
  //Message used for writing messages to services
  Message     interface{}  `json:"message,omitempty"`
  //MessageID used for pulling messages from topics
  MessageID   int          `json:"message_id,omitempty"`
}
```
### Endpoints
|Verbs| Definition|
|-|-|
|Obtain| Get existing or create|
|Create | Create new or error if already exists|
|Fetch  | Get existing or error if does not exists|
|Write  | Write data to server|
|Pull   | Read information from server by http request (pull) after subscribing to a pull agreement of event data|
|Subscribe/Unsubscribe | Setup or delete push/pull agreement|

|Endpoint|Use|Params|
|-|-|-|
|`/users/user/obtain`|Explicitly creates a User and returns the User object. Returns the user UUID|Mandatory fields only|
|`/topics/topic/subscribe`|Subscribe to an existing Topic. Returns the subscription detail and status|topic, [*webhook_url*] (if requesting push subscription)|
|`/topics/topic/unsubscribe`|Unsubscribe from an existing topic. Returns the subscription detail with status (unsubscribed if successful)|topic|
|`/topics/topic/create`|Explicitly create a topic with a given topic name. Returns the topic information or error if already exists|topic|
|`/topics/fetch`|Returns a list of topics that can be subscribed to by the User|Mandatory fields only|
|`/topics/topic/fetch`|Explicitly fetch a topic with a given topic name. Returns the topic information of error if topic does not exist |topic|
|`/topics/topic/obtain`|Get an existing topic of a given name of create a topic with that name if one does not exist. Returns topic information|topic|
|`/topics/topic/messages/pull`|Get a message from a topic's message queue. Messages start at pointer position 1|topic, message_id|
|`/topics/topic/messages/write`|Write a message to a topic queue|topic, message|


## Limitations
1. Only the creator **User** of a topic can write to it
1. A **User** is a disposable object that identifies credentials associated with a group of subscriptions. They are deleted when they are no longer associated with subscriptions. They are created passively when a username/password pair are used to subscribe or create a topic, so long as username does not exist already (failed request). In that case the User will either be logged in (if password matches) or the request will fail due to an unauthorised request.
1. When a **Topic** no longer has any subscribers, it is deleted. Topics can be passively created again if any user attempts to write to the topic or actively by sending a request to the `/topic/create` endpoint. In which case that user will become the creator of the topic, and the only User authorised to write to it. This should not cause issues as the creator of a topic is automatically subscribed to it, so must actively unsubscribe, or allow the subscription to go stale and be tombstoned by not consuming the stream. As a failsafe, create a new user to consume the topic by webhook to keep alive.
1. **Messages** that have been consumed and acknowleged by all subscribers are deleted. 
1. Add a pushURL/WebhookURL to subscribe as a push subscriber. Otherwise you will have to pull the message via retrieval endpoint with a messageID to get the next message. You cannot mix methods or change subscription type after initial subscripton, without first unsubscribing and subscribing again.

## Roadmap
- [x] Timed garbage collection of stale subscribers by first tombstoning subscribers preventing deletion of tickets over 50 places behind PointerHead, then deleting them on second pass if they are still there after a certain amount of time. Designed to prevent inactive subscribers forcing long term storage of messages (to maintain a stable service)
- [x] Timed pushing of messages to Webhook URLs
- [x] Implement deletion of User when no longer subscribed to any topic
- [x] Implement deletion of Topic when no longer has any subscribers
- [x] Implement Errors being parsed back to client via standard error object rather than status 5xx pages 
- [ ] Persist messages in key-value database for disaster recovery
- [ ] Error managment for issues that come up in `pubsub.metranome`
- [ ] Include a Docker file for easy deployment
- [ ] Inlude a Cloud Build YAML file for easy CICD to GCP Cloud Run
- [ ] Add a Bash script for easy local deploy for testing using Buildah and Podman
