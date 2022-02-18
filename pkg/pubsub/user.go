package pubsub

import (
	"fmt"
	"net/url"
)

//Subscribe method subscribes the user to the given topic using
// the given pushURL. If no pushURL, subscription is pull type
// using the topic ID.
func (user *User) Subscribe(topic *Topic, pushURL string)error{
  //check pushURL is valid
  var url *url.URL
  var err error
  if pushURL != ""{
    url,err=url.Parse(pushURL)
    if err!= nil{return fmt.Errorf("push URL not valid: %v",err)}
  }
  //Create Subsriber Object
  sub := Subscriber{
    ID: user.UUID,
    User: user,
    PushURL: url ,
  }
  user.mu.Lock()

  //add to User subscriber list
  user.Subscriptions[topic.ID] = pushURL 
  //Add subscriber object to topic to receive messages from
  // current head position
  if _,ok:=topic.PointerPositions[topic.PointerHead];!ok{
    topic.PointerPositions[topic.PointerHead] = make(Subscribers)
  }
  topic.PointerPositions[topic.PointerHead][sub.ID]=sub

  user.mu.Unlock()

  return nil
}

//Unsubscribe helper function to unsubscribe a user from a topic
func (user *User) Unsubscribe(topic *Topic)error{
  user.mu.Lock()
  //remove from User subscriber list
  delete(user.Subscriptions,topic.ID)
  //remove from topic pointerPosition list to no
  // longer receive messages
  delete(topic.PointerPositions[topic.PointerHead],user.UUID)
  user.mu.Unlock()
  return nil
}

//WriteToTopic
func (user *User) WriteToTopic(topic *Topic,message Message)(Message,error){
  //check user is the creator of the topic
  if user.UUID != topic.Creator.UUID{
    return Message{},fmt.Errorf("User does not have the authorisation to write to this channel")
  }
  user.mu.Lock()
  //Add message to topic's message queue
  topic.PointerHead+=1
  message.ID = topic.PointerHead
  topic.Messages[topic.PointerHead] = message 
  user.mu.Unlock()
  return message ,nil
}
//PullMessage retrieves a message from the Topic message queue if the user is subscibed
func (user *User) PullMessage(topic *Topic, messageID int)(Message,error){
  //check user is subscribed and isn't pulling a push sub
  pushURL,ok:=user.Subscriptions[topic.ID]
  if !ok{
    return Message{},fmt.Errorf("User not subscribed to Topic")
  }else if pushURL!=""{
    return Message{}, fmt.Errorf("Not Allowed. User attempting to pull from push subscription")
  }
  //get message from position if exists
  if msg,ok:=topic.Messages[messageID];ok{
    //Move pointer
    for pos,subs:=range topic.PointerPositions{
      if s,ok:=subs[user.UUID];ok{
        if  messageID > pos{
          break
        }
        user.mu.Lock()
        topic.PointerPositions[messageID][user.UUID] = s
        delete(topic.PointerPositions[pos],user.UUID)
        user.mu.Unlock()
      }
    }
    return msg,nil
  }else{
    return Message{},fmt.Errorf("This message does not exist. Head point is %d",topic.PointerHead)
  }
  
}