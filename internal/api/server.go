package api

import (
	"fmt"
	"net/http"
)

// Server is the HTTP API server scaffold for v1-final.
// In v1-alpha, all routes return 501 Not Implemented.
type Server struct {
	host string
	port int
	srv  *http.Server
}

// NewServer creates a new HTTP API server.
func NewServer(host string, port int) *Server {
	return &Server{host: host, port: port}
}

// Start begins listening. Returns error if unable to start.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleAll)

	s.srv = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.host, s.port),
		Handler: mux,
	}

	return s.srv.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() error {
	if s.srv != nil {
		return s.srv.Close()
	}
	return nil
}

func (s *Server) handleAll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	fmt.Fprint(w, `{"error":"not implemented","message":"HTTP API will be available in v1-final"}`)
}
