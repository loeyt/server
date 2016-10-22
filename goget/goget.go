// Package goget implements a server.Service that serves go-import meta tags
// for vanity imports of Go code.
package goget

import (
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"loe.yt/server"
)

// Import holds the data needed for go-gettable endpoints. Prefix, Vcs and
// Repo are the fields needed to serve a go-import meta-tag. Vcs and Repo are
// what you would expect them to be. Prefix has one gotcha: it's without the
// host part of the import prefix. So for "loe.yt/server/cmd/loeyt-server"
// this should be "server/cmd/loeyt-server". The host part is added from the
// Host header of the HTTP request.
//
// Requests lacking the "go-get=1" query will be redirected to Redirect. If
// Redirect is empty then the redirect is targeted at godoc.org.
type Import struct {
	Prefix   string
	Vcs      string
	Repo     string
	Redirect string
}

// ImportSource describes the interface this service expects to fetch the data
// needed to serve go-import metadata and redirects.
type ImportSource interface {
	GetImport(u *url.URL) (*Import, error)
}

// Static is an ImportSource with a static mapping of path to Import. The path
// should be trimmed of any leading and trailing slashes, and is without the
// host part of the import path.
type Static map[string]*Import

// GetImport takes u.Path and returns the Import with that key (nil if not
// present).
func (s Static) GetImport(u *url.URL) (*Import, error) {
	prefix := strings.Trim(u.Path, "/")
	return s[prefix], nil
}

type service struct {
	ImportSource
}

// NewService creates a new server.Service using the supplied ImportSource to
// find valid go-get imports.
func NewService(s ImportSource) server.Service {
	return &service{ImportSource: s}
}

func (s *service) MatchHTTP(r *http.Request) (bool, error) {
	i, err := s.GetImport(r.URL)
	if err != nil {
		return false, err
	}
	return i != nil, nil
}

var metaTmpl = template.Must(template.New("meta").Parse(
	`<meta name="go-import" content="{{.Host}}/{{.Prefix}} {{.Vcs}} {{.Repo}}">
`))

func (s *service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	i, err := s.GetImport(r.URL)
	if err != nil {
		// TODO: add logging
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if r.URL.Query().Get("go-get") == "1" {
		data := map[string]string{
			"Host":   r.Host,
			"Prefix": i.Prefix,
			"Vcs":    i.Vcs,
			"Repo":   i.Repo,
		}
		if err := metaTmpl.Execute(w, data); err != nil {
			// TODO: add logging
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	} else {
		target := "https://godoc.org/" + r.Host + r.URL.Path
		if i.Redirect != "" {
			target = i.Redirect
		}
		http.Redirect(w, r, target, http.StatusFound)
	}
}
