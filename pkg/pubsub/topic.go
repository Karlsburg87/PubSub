package pubsub

//MessageCleanup 1) deletes old messages in the array queue prior to the oldest subscriber pointer 2)tags inactive subscribers for removal 3)removes old inactive subscribers
//
//Enables message garbage collection by clearing inactive subscriptions blocking cleanup
func (topic *Topic) MessageCleanup(){
  
}

