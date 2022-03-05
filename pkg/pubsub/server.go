package pubsub

import (
	"fmt"
	"net/http"
	"time"
)

//CreateServer provides the standard HTTP server for the application.
// Intended for use with CreateMux
//
//Note: WriteTimeout has been disabled (was: 5)
// to allow for correct Server Sent Events functionality.
// In future a secondary server should be employed in a
// goroutine for SSE to maintain stricter timeout
// controls on other requests
func CreateServer(port int, mux *http.ServeMux) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		IdleTimeout:       10 * time.Second,
		WriteTimeout:      0 * time.Second,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		Handler:           mux,
	}
}
