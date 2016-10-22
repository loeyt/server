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

// Import
type Import struct {
	Prefix   string // the import prefix, without the host part
	Vcs      string // the version control system
	Repo     string // the repository root
	Redirect string // redirect target for non-go-get requests ("" = Repo)
}

type ImportSource interface {
	GetImport(u *url.URL) (*Import, error)
}

// Static is an ImportSource with a static mapping of path to Import.
type Static map[string]*Import

func (s Static) GetImport(u *url.URL) (*Import, error) {
	prefix := strings.Trim(u.Path, "/")
	return s[prefix], nil
}

type service struct {
	ImportSource
}

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
