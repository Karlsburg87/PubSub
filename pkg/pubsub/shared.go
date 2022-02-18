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
		UsernameHash: fmt.Sprintf("%x", u.Sum(nil)),
		PasswordHash: fmt.Sprintf("%x", p.Sum(nil)),
    mu: &sync.Mutex{},
    Subscriptions: make(map[int]string),
	}
	user.UUID = fmt.Sprintf("%x", u.Sum([]byte(password)))

	return user, nil
}

//getHTTPData takes an incoming request and creates a map of data within BOTH the URL 
// query params and the JSON body payload. Priority to the URL query params data if
// conflicts present
func getHTTPData(req *http.Request)(IncomingReq,error){
  m:=IncomingReq{}
  //add JSON payload from body
  bod,err:=io.ReadAll(req.Body)
  if err!=nil{
    return IncomingReq{},err
  }
  if len(bod)>0 &&bod!=nil{
    err := json.Unmarshal(bod,&m)
    if err!=nil{
      return IncomingReq{},err
    }
  }
  //add URL queries
  query:=req.URL.Query()
  for k,v:=range query{
    if v[0] ==""{continue}
    switch strings.ToLower(k){
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
        m.MessageID,err = strconv.Atoi(v[0])
        if err!=nil{
          return IncomingReq{},err
        }
    }
  }
  return m,nil
}

//HTTPErrorResponse responds correctly to http request
// errors in the handler function
func HTTPErrorResponse(err error,errType int ,rw http.ResponseWriter)error{
  if err!=nil{
    log.Println(err)
    rw.WriteHeader(errType)
    if _,err:=rw.Write([]byte(err.Error()));err!=nil{
      log.Panicln(err)
    }    
    return err
  }
  return nil
}

//HTTPAuthenticate does the boilerplate check username and password work for incoming service queries
func HTTPAuthenticate(rw http.ResponseWriter,r *http.Request,pubsub *PubSub)(*User, IncomingReq, error){
    payload,err:=getHTTPData(r)
    if err!=nil{
      return nil, payload,HTTPErrorResponse(err,http.StatusInternalServerError,rw)
    }
    //Check there is a username and password
    if payload.Username=="" || payload.Password==""{
        if err!=nil{
          log.Println("Request did not pass full login credentials. Missing Username or Password")
          rw.WriteHeader(http.StatusBadRequest)
          if _,err:=rw.Write([]byte(err.Error()));err!=nil{
            log.Println(err)
            return nil,payload,err
          }    
          return nil,payload,err
      }
    }
    //login or create user
    user,err:= pubsub.GetUser(payload.Username,payload.Password)
    return user, payload,HTTPErrorResponse(err,http.StatusInternalServerError,rw) 
}

//respondMuxHTTP is the standard responder to mux handler workloads
func respondMuxHTTP(rw http.ResponseWriter,res Responder){
  response,err := res.toJSON()
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
  //respond
  rw.Header().Set("content-type","application/json")
  _,err=fmt.Fprintf(rw,"%s",string(response))  
  if err:=HTTPErrorResponse(err,http.StatusInternalServerError,rw);err!=nil{return}
}