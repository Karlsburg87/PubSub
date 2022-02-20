package pubsub

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

//createNewUser creates a new user from a given
// unhashed username and password string
func createNewUser(username, password string) (*User, error) {
	//hash entries
	u := sha256.New()
	_, err := u.Write([]byte(username))
	if err != nil {
		return nil, err
	}
	p := sha256.New()
	_, err = u.Write([]byte(password))
	if err != nil {
		return nil, err
	}

	user := &User{
		UsernameHash:  fmt.Sprintf("%x", u.Sum(nil)),
		PasswordHash:  fmt.Sprintf("%x", p.Sum(nil)),
		mu:            &sync.Mutex{},
		Subscriptions: make(map[string]string),
	}
	user.UUID = fmt.Sprintf("%x", u.Sum([]byte(password)))
	user.AddCreatedDatestring(time.Now())
	return user, nil
}

//getHTTPData takes an incoming request and creates a map of data within BOTH the URL
// query params and the JSON body payload. Priority to the URL query params data if
// conflicts present
func getHTTPData(req *http.Request) (IncomingReq, error) {
	m := IncomingReq{}
	//add JSON payload from body
	bod, err := io.ReadAll(req.Body)
	if err != nil {
		return IncomingReq{}, err
	}
	if len(bod) > 0 && bod != nil {
		err := json.Unmarshal(bod, &m)
		if err != nil {
			return IncomingReq{}, err
		}
	}
	//add URL queries
	query := req.URL.Query()
	for k, v := range query {
		if v[0] == "" {
			continue
		}
		switch strings.ToLower(k) {
		case "username":
			m.Username = v[0]
		case "password":
			m.Password = v[0]
		case "message":
			m.Message = v[0]
		case "topic":
			m.Topic = v[0]
		case "webhook_url":
			m.WebhookURL = v[0]
		case "message_id":
			m.MessageID, err = strconv.Atoi(v[0])
			if err != nil {
				return IncomingReq{}, err
			}
		}
	}
	return m, nil
}

//HTTPErrorResponse responds correctly to http request
// errors in the handler function
func HTTPErrorResponse(err error, errType int, rw http.ResponseWriter) error {
	if err != nil {
		log.Printf("%v : (HTTP Status Code: %d)\n", err, errType)
		rw.WriteHeader(errType)
		//wrap error message
		errResponse := map[string]string{
			"error": err.Error(),
		}
		out, errMarshall := json.MarshalIndent(errResponse, " ", " ")
		if errMarshall != nil {
			log.Panicln(fmt.Errorf("Error marshalling json in HTTPErrorResponse: %v", err))
		}
		rw.Header().Set("content-type", "application/json")
		if _, err := fmt.Fprint(rw, string(out)); err != nil {
			log.Panicln(fmt.Errorf("Error writing to rw.Write in HTTPErrorResponse: %v", err))
		}
		return err
	}
	return nil
}

//HTTPAuthenticate does the boilerplate check username and password work for incoming service queries
//
//IncomingReq is the rolled up query including fields from
// body json and url query args (see getHTTPData)
func HTTPAuthenticate(rw http.ResponseWriter, r *http.Request, pubsub *PubSub) (*User, IncomingReq, error) {
	//get data from URL query string and JSON body
	payload, err := getHTTPData(r)
	if err != nil {
		return nil, payload, HTTPErrorResponse(err, http.StatusInternalServerError, rw)
	}
	//Check there is a username and password
	if payload.Username == "" || payload.Password == "" {
		log.Println("Request did not pass full login credentials. Missing Username or Password")
		if err := HTTPErrorResponse(fmt.Errorf("Username and password must be given as request parameters"), http.StatusBadRequest, rw); err != nil {
			fmt.Println("this ran")
			return nil, payload, err
		}
	}
	//login or create user
	user, err := pubsub.GetUser(payload.Username, payload.Password)
	if err := HTTPErrorResponse(err, http.StatusBadRequest, rw); err != nil {
		return nil, payload, err
	}

	return user, payload, HTTPErrorResponse(err, http.StatusInternalServerError, rw)
}

//respondMuxHTTP is the standard responder to mux handler workloads
func respondMuxHTTP(rw http.ResponseWriter, res Responder) {
	response, err := res.toJSON()
	if err := HTTPErrorResponse(err, http.StatusInternalServerError, rw); err != nil {
		return
	}
	//respond
	rw.Header().Set("content-type", "application/json")
	_, err = fmt.Fprintf(rw, "%s", string(response))
	if err := HTTPErrorResponse(err, http.StatusInternalServerError, rw); err != nil {
		return
	}
}

//isStale checks whether the given date to check is considered stale
//
//- timeToCheck is the time to check for staleness
//
//- consideredStale is the timeframe after timeToCheck that the timeToCheck could be considered stale
//
//Intended use is for tombstoning activity
func isStale(timeToCheck time.Time, consideredStale time.Duration) bool {
	return timeToCheck.Add(consideredStale).Before(time.Now())
}
