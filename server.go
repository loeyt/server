// Package server contains the core of the loe.yt server implementation.
package server // import "loe.yt/server"

import "net/http"

// Service is a http.Handler which can inspect the request and decline
// handling it through the MatchHTTP method.
type Service interface {
	MatchHTTP(r *http.Request) (bool, error)
	http.Handler
}

// Handler is a http.Handler that goes through a list of Service handlers,
// picking the first one that matches itself against the request.
type Handler struct {
	Services []Service
}

// ServeHTTP makes Handler a http.Handler, and iterates through h.Services to
// find the first Service that can handle the request.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, s := range h.Services {
		match, err := s.MatchHTTP(r)
		if err != nil {
			// TODO: add logging
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if match {
			s.ServeHTTP(w, r)
			return
		}
	}
	http.NotFound(w, r)
}
