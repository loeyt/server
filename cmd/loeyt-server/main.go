package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/context"
	"loe.yt/server"
	"loe.yt/server/goget"
)

const leStagingURL = "https://acme-staging.api.letsencrypt.org/directory"

var (
	serverImport = &goget.Import{
		Prefix:   "server",
		Vcs:      "git",
		Repo:     "https://github.com/loeyt/server",
		Redirect: "https://github.com/loeyt/server",
	}
	goVersionImport = &goget.Import{
		Prefix:   "go-version",
		Vcs:      "git",
		Repo:     "https://github.com/loeyt/go-version",
		Redirect: "https://github.com/loeyt/go-version",
	}

	dispatcher = &server.Handler{
		Services: []server.Service{
			server.Redirect("https://luit.eu/", http.StatusFound,
				"/",
			),
			goget.NewService(goget.Static{
				"server":                  serverImport,
				"server/cmd":              serverImport,
				"server/cmd/loeyt-server": serverImport,
				"go-version":              goVersionImport,
			}),
		},
	}

	drone = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "172.17.0.2:8000"
			if _, ok := req.Header["User-Agent"]; !ok {
				// explicitly disable User-Agent so it's not set to default value
				req.Header.Set("User-Agent", "")
			}
			req.Header.Set("X-Forwarded-Proto", "https")
		},
	}
)

func main() {
	l := listener()
	err := http.Serve(l, handler{
		"loe.yt":       dispatcher,
		"drone.loe.yt": drone,
	})
	if err != nil {
		log.Fatalln(err)
	}
}

func listener() net.Listener {
	fds := listenFds()
	if fds == nil {
		log.Println("listening on :8080")
		l, err := net.Listen("tcp", ":8080")
		if err != nil {
			log.Fatal(err)
		}
		return l
	}
	if len(fds) < 1 {
		log.Fatalln("got LISTEN_FDS=0")
	}
	if len(fds) > 1 {
		serveHTTP(fds[1])
	}
	l, err := net.FileListener(fds[0])
	if err != nil {
		log.Fatalln("error using fd as net.Listener:", err)
	}
	return tls.NewListener(l, tlsConfig())
}

// serveHTTP serves a http to https redirector.
func serveHTTP(f *os.File) {
	l, err := net.FileListener(f)
	if err != nil {
		log.Fatalln("error using fd as net.Listener:", err)
	}
	go http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://"+r.Host+r.URL.Path, http.StatusPermanentRedirect)
	}))
}

func listenFds() []*os.File {
	nfds, err := strconv.Atoi(os.Getenv("LISTEN_FDS"))
	if err != nil {
		return nil
	}
	fds := make([]*os.File, nfds)
	for n := range fds {
		name := fmt.Sprintf("LISTEN_FDS_%d", n)
		fds[n] = os.NewFile(uintptr(n+3), name)
	}
	return fds
}

var manager *autocert.Manager

func tlsConfig() *tls.Config {
	if manager == nil {
		manager = &autocert.Manager{
			Prompt: autocert.AcceptTOS,
		}
		whitelist := strings.Split(os.Getenv("ACME_WHITELIST"), ":")
		if len(whitelist) > 1 || whitelist[0] != "" {
			fmt.Println("allowing hosts", whitelist)
			manager.HostPolicy = autocert.HostWhitelist(whitelist...)
		} else {
			fmt.Println("using Let's Encrypt Staging environment")
			manager.Client = &acme.Client{
				DirectoryURL: leStagingURL,
			}
			manager.HostPolicy = func(ctx context.Context, host string) error {
				return nil
			}
		}
		cache := os.Getenv("ACME_CACHE")
		if cache == "" {
			cache = "."
		}
		manager.Cache = autocert.DirCache(cache)
	}
	return &tls.Config{
		GetCertificate: manager.GetCertificate,
	}
}

type handler map[string]http.Handler

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if hh, ok := h[r.Host]; ok {
		hh.ServeHTTP(w, r)
	} else {
		http.NotFound(w, r)
	}
}
