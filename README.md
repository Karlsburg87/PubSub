[![Go Report Card](https://goreportcard.com/badge/github.com/CDennis-CR/PubSub)](https://goreportcard.com/report/github.com/CDennis-CR/PubSub)
[![Run on Repl.it](https://repl.it/badge/github/CDennis-CR/PubSub)](https://repl.it/github/CDennis-CR/PubSub) 

![Project Status](https://img.shields.io/badge/Status-Early%20Development-yellow?style=flat-square&cacheSeconds=3600)

# PubSub
An ***RESTful-like*** HTTP service to make signing up for event streams easy and open to anyone able to access the URI endpoint. Useful for passing in things like uptime stats.

> Pubsub guarentees '*at least once*' message delilvery - up until the subscription to the topic becomes *stale* after a period of inactivity

Acknowledgement based system to ensure message delivery guarentees are met.
- Push Subscriptions (Webhooks) need return a 200 or 201 status code to acknowlege. 
- Message pull subscriptions acknowlege message receipt of earlier pointer positions when requesting a later pointer position.

### RESTful-like?
This implementation aims to be familiar for people used to integrating RESTful services without being strictly compliant with any common definition.

The API is designed to be fully accessible purely through GET HTTP requests, in order to make it open to clients that do not have ready access to the full set of HTTP CRUD methods (e.g. browsers, webtool UIs, spreadsheets, etc). 

To make this work, a custom verb list is used for appending to standardised endpoints. All arguments can be given through query parameters on static general URI paths.

If you prefer to pass arguments via JSON in the request body, you can still do so using the same static endpoints - or mix the two options.

## Project Goals
This project aims to provide a performant publish and subscribe server for use in local testing and small-to-medium production workloads where event messages are non sensitive. To be easily parachuted into projects with simple deployment, memorable API and SSE interface for web apps.

In priority order:

> Goal 1: Simple to run

- Packaged in single Dockerfile.
- Evaluate on Replit with a click.

Deploy with a `docker run` command and config with command line args or envars if you want to.

Familiar to newcomers with some knowledge of APIs, with an interface simple and small enough to fit into working memory.

> Goal 2: Accessible to use

Initially built for delivering open data to public consumers, focus is on open accessibility. 
- Easy to discover and subscribe to message streams with a single command.
- Easy to publish streams to websites using SSE

> Goal 3: Fast & efficient

The aim is to be quick *enough* and not use excessive resource. Our aim is to run a throughput of 60MB per second with a latency of 25 ms (200 MB/s load) on a Raspberry pi with resource to spare.

[Competitor benchmarks](https://www.confluent.io/en-gb/blog/kafka-fastest-messaging-system/)

PubSub Benchmarks to follow (*see Roadmap section*)

## Status
**PubSub** is in initial development (v0.xx) stage and subject to constant change to its API.

**Not yet suitable for deployment in production environments**

## Data Persistance
Pubsub is an in-memory system, however it persists message data to a file-based KV database and blob stores in future to assist with disaster recovery efforts.

When implemented, messages are stored in JSON format within a local blob store with file name convention: `{topicName}/{messageID}`

Both subscriber and user lists are stored in GOB format within a local BoltDB KV store with:

 - User keys convention: `user/{userID}`

 - Subscriber keys convention: `sub/{topicName}/{messageID}/{subscriberID}`

Data storage shadows the in memory workflow and will only be called in the event of disaster recovery.

All persisted data will be in the directory `/store` when run via the docker container with defaults.This can be persisted from the Docker container using volumes such as using the command ` docker run --volumes-from [...]`. To change the location, set the environment variable `PS_STORE`.

## Usage
### Request Params
Parameters can be sent by URL query, HTTP Post  JSON payload or a mixture of both.

example URL encoded:
```http
https://some.endpoint/users/user/obtain?username=usrname&password=pswrd
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
### Verbs
|Verbs| Definition|
|-|-|
|Obtain| Get existing or create|
|Create | Create new or error if already exists|
|Fetch  | Get existing or error if does not exists|
|Write  | Write data to server|
|Pull   | Read information from Topic's Message queue by http request (pull) after creating a pull subscription to a Topic|
|Subscribe | Setup a push/pull association to a Topic|
|Unsubscribe|Remove an push/pull association to a Topic|

### Endpoints
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

## Settings
PubSub is configured through use of environemnt variables. The following are a list of configurable options. 

Note that the Dockerfile comes with best practise defaults that can be overridden in the `docker build` step using the same variable names and the `--build-arg` flag
|Environment Variable|Usaage|Default|
|-|-|-|
|`PS_STORE`|The root directory where PubSub data will be persisted to for the purposes of disaster recovery|'store/'|
|`PS_SUPERADMIN_USERNAME`|The username of an initial user - created automatically on startup. Despite the name Superadmin is a regular user and exists as an initialiser. Will be garbage collected if not used operationally. Can be easily altered to have admin privilidges by a developer|'ping'|
|`PS_SUPERADMIN_PASSWORD`|Password of the initial user. If not set a random string will be used. This effectively makes user ping unusable as the password is not printed to stdout|random alphanumeric string|
|`PS_DURATION_STALE`|The time allowed before an orphaned object is tombstoned. A duration string format* |'3h'|
|`PS_DURATION_RESURRECT`|The time allowed after tombstoning before a deletion is committed. A duration string format|'30m'|

<pre> [*] A duration string is a possibly signed sequence of decimal numbers, each with optional fraction and a unit suffix, such as "300ms", "-1.5h" or "2h45m". Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".</pre>

## Object Life Cycles & Limitations
### Users
> A **User** is a disposable object that identifies credentials associated with a group of subscriptions.

1. They are garbage collected when they are no longer associated with subscriptions.
1. They are created passively when a username/password pair are used to subscribe or create a topic, so long as username does not exist already (this results in a failed request with an unauthorised access header).
1. If successfull the **User** will either be logged in to an existing **User** (if password matches) or a new **User** created and immediately logged in to perform the action.
1. You do not need to login explicitly using the `/users/user/obtain` endpoint, but it may be useful to check when the **User** was created or see which Topics it is subscribed to.
  
### Topics 
> A **Topic** is a is a container for a stream of related messages.

1. A **Topic** can be passively created if any **User** attempts to write to the topic by sending a request to the `topics/topic/create` endpoint. In which case that user will become the creator of the topic, and the only **User** authorised to write to it.
    1. You can check who is the creator of an existing **Topic** without fear of passively creating it by using the `/topics/fetch` endpoint 
1. When a **Topic** no longer has any subscribers, it is garbage collected.
    1.  This should not cause issues as the **User** that is the creator of a **Topic** is automatically subscribed to it. So by default a new **Topic** will have 1 subscriber.
1.  The creator's **Subscription** is always at the head position for a **Topic** to which it is the creator even without consuming or acknowledging any messages -and will not get in the way of tombstoning and garbage collection of old messages.
    1.  The **Subscription** can still go stale, be tombstoned and eventually garbage collected by not writing to the stream. This will allow the Topic to be garbage collected as it will no longer have subscribers. You can prevent this by writing to the stream again - which will remove the active tombstones.

### Subscription
> A **Subscription** is an association of a **User** with a **Topic** to which the User wishes to be updated of new incoming **Messages**. It is used to maintain the position of the next message yet to be read by the **User** in the **Topic** queue.

1. A **Subscription** is garbage collected if no new read acknoledgements have been received before the 'gone Stale' deadline.
    1. This can be an issue when the **Topic** publishes messages at a very low frequency and the subscription is `push`. However it is necarssary to garbage collect inactive **Topics**. Configurations to the garbage collector (tombstoner config) can remedy this.   

### Messages
> A **Message** is the unit of data published to the **Topic** by the publisher to be consumed by the subscriber

1. **Messages** that have been consumed and acknowleged by all subscribers are garbage collected. 
1. Add a pushURL/WebhookURL to subscribe as a push subscriber. Otherwise you will have to pull the message via retrieval endpoint with a messageID to get the next message. You cannot mix methods or change subscription type after initial subscripton, without first unsubscribing and subscribing again.
    1. You can do this in one action by using the `/topics/topic/subscribe` endpoint. However the subscription pointer will move to the Topic's head position and previous messages may become unobtainable.

## Gotchas
-Head pointer positions are zero indexed and 1 above the number of messages the Topic has had published to it. This is in line with message IDs which are also zero indexed. So a topic with no messages will have pointer head at 0. But a Topic with 10 messages will have a PointerHead at position 10 with the latest message having an ID of 9. Subscriptions at Pointer reference 10 are awaiting message with ID 10 which will be the next message to be published.    
-The user that creates a new topic is automatically subscribed to it and is at the head pointer position. Therefore it is keeping the topic alive even if there are no other subscribers as it can never be garbage collected 
- Subscriptions at head pointer position are not garbage collected as they are considered active and awaiting new messages. This also keeps the User alive.
- The creator subscriber is automatically moved to the pointerHead on every write without needing to consume and acknoledge the message.
- Even if unsubscribed, the creator subuser is resubscribed on each write to the topic. 
- If a user has unsubscribed from a topic it has created and does not resubscribe or write by the time the application is stopped, the restore functionality will default the creator of the Topic to the new superadmin rather than back to the original creator. This means the original creator can not write further messages to the Topic. This is intentional behaviour akin to restoring a Topic in archive mode and preventing resulting object orphening or unexpected  cascading of object deletions by the garbage collector (Topic > Subscribers > Users not subscribed to other Topics)
- On restore, the new superAdmin is automatically subscribed to everything at pointerHead 0. As no Topics without messages are restored this will mean the superAdmin User will be garbage collected if it does not consume messages -which it is unlikely to do. So, if you are wondering what the spate of subscriber deletions of the same ID are after a restore - it is propably your superAdmin subscriptions being cleaned up. 
- To delete a topic, all subscribers including the creator must unsubscribe if their subscription pointer is at the pointerHead position. Subscriptions which are inactive in other pointer positions where messages cannot be delivered are garbage collected anyway.

## Roadmap
- [x] Timed garbage collection of stale subscribers by first tombstoning subscribers preventing deletion of tickets over 50 places behind PointerHead, then deleting them on second pass if they are still there after a certain amount of time. Designed to prevent inactive subscribers forcing long term storage of messages (to maintain a stable service)
- [x] Timed pushing of messages to Webhook URLs
- [x] Implement deletion of User when no longer subscribed to any topic
- [x] Implement deletion of Topic when no longer has any subscribers
- [x] Implement Errors being parsed back to client via standard error object rather than status 5xx pages 
- [x] Persist messages in key-value database for disaster recovery
- [ ] Error managment for issues that come up in `pubsub.metranome`
- [x] Include a Dockerfile/Containerfile for easy deployment
- [ ] Inlude a Cloud Build YAML file for easy CICD to GCP Cloud Compute Engine
- [x] Add a Bash script for easy local deploy for testing using Buildah and Podman
- [x] Server Sent Events implementation for websites wanting to consume streams to display directly in the client UI
- [x] Implement front end web app for onboarding new users
- [ ] Benchmarking
- [ ] Unit tests
- [ ] Inform Push subscribers when their subscriptions have been garbage collected as this may be due to low frequency publishing rates of the Topic rather than inactive subscribers.
- [x] Add ability to config the tombstoner deadlines.
- [x] Fix User already exists bug that effeects users after restore from persistance layer 