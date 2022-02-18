package pubsub

import (
	"fmt"
	"net/http"
	"time"
)

//CreateServer provides the standard HTTP server for the application.
// Intended for use with CreateMux
func CreateServer(port int, mux *http.ServeMux) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		IdleTimeout:       10 * time.Second,
		WriteTimeout:      5 * time.Second,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		Handler:           mux,
	}
}
