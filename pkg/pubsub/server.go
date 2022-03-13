package pubsub

import (
	"fmt"
	"net/http"
	"time"
)

//ServerType is an enum allowing choice of server setup for either the API or Server Side Events (SSE)
type ServerType int

const (
	//ServerSSE is the setup for Server Side Events servers - notably with WriteTimeout disabled
	ServerSSE ServerType = iota
	//ServerAPI is the config for the API server
	ServerAPI
)

//CreateServer provides the standard HTTP server for the application.
// Intended for use with CreateMux
//
//Note: WriteTimeout has been disabled (was: 5)
// to allow for correct Server Sent Events functionality.
// In future a secondary server should be employed in a
// goroutine for SSE to maintain stricter timeout
// controls on other requests
func CreateServer(port int, stype ServerType, mux *http.ServeMux) *http.Server {
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		IdleTimeout:       10 * time.Second,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		Handler:           mux,
	}

	if stype == ServerSSE {
		server.WriteTimeout = 0 //disable timeout with zero if using for SSE to avoid closing connections between messages
	} else if stype == ServerAPI {
		server.WriteTimeout = 5 * time.Second
	}

	return server
}
