// Package server contains the core of the loe.yt server implementation.
package server // import "loe.yt/server"

import "net/http"

// Service is a http.Handler which can inspect the request and decline
// handling it through the MatchHTTP method.
type Service interface {
	MatchHTTP(r *http.Request) (bool, error)
	http.Handler
}

// Redirect returns a Service that redirects the set of URL.Path values in
// match to the specified location using the given HTTP status code.
func Redirect(location string, code int, match ...string) Service {
	return &redirect{
		Location: location,
		Code:     code,
		Match:    match,
	}
}

type redirect struct {
	Location string
	Code     int
	Match    []string
}

func (r *redirect) MatchHTTP(req *http.Request) (bool, error) {
	for _, m := range r.Match {
		if m == req.URL.Path {
			return true, nil
		}
	}
	return false, nil
}

func (r *redirect) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, r.Location, r.Code)
}

// Handler is a http.Handler that goes through a list of Service handlers,
// picking the first one that matches itself against the request.
type Handler struct {
	Services []Service
}

// ServeHTTP makes Handler a http.Handler, which iterates through h.Services
// to find the first Service that can handle the request.
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
