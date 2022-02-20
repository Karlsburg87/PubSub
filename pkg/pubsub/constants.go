package pubsub

//Enum for what type of HTTP Verb was used
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
