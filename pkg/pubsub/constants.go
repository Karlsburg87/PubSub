package pubsub

//verbType is an Enum for what type of HTTP Verb was used
type verbType int

const (
	obtainVerb verbType = iota
	createVerb
	fetchVerb
	writeVerb
	pullVerb
	subscribeVerb
	unsubscribeVerb
)

//PersistUnit is an Enum type for what needs to be persisted for the defulat Persit implementation for streamers
type PersistUnit int

const (
	//PersistUser gives an enum option for User using the PersistUnit type
	PersistUser PersistUnit = iota
	//PersistSubscriber gives an enum option for Subscriber using the PersistUnit type
	PersistSubscriber
)
