package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"

	"loe.yt/server"
	"loe.yt/server/goget"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/context"
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

	tlsListener = false

	drone = &httputil.ReverseProxy{
		Director: droneDirector,
	}
	droneWs = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isWebsocket(r) {
			droneDirector(r)
			droneHandleWebsocket(w, r)
			return
		}
		drone.ServeHTTP(w, r)
	})
)

func isWebsocket(r *http.Request) bool {
	if strings.ToLower(r.Header.Get("Connection")) == "upgrade" {
		return strings.ToLower(r.Header.Get("Upgrade")) == "websocket"
	}
	return false
}

func droneDirector(r *http.Request) {
	r.URL.Scheme = "http"
	r.URL.Host = "172.17.0.2:8000"
	if _, ok := r.Header["User-Agent"]; !ok {
		// avoid adding a User-Agent, if none exists
		r.Header.Set("User-Agent", "")
	}
	if tlsListener {
		r.Header.Set("X-Forwarded-Proto", "https")
	}
}

func droneHandleWebsocket(w http.ResponseWriter, r *http.Request) {
	bc, err := net.Dial("tcp", "172.17.0.2:8000")
	if err != nil {
		// TODO: add logging
		http.Error(w, "error contacting backend server", http.StatusBadGateway)
		return
	}
	defer bc.Close()
	hj, ok := w.(http.Hijacker)
	if !ok {
		// TODO: add logging
		http.Error(w, "unable to hijack connection", http.StatusInternalServerError)
		return
	}
	c, _, err := hj.Hijack()
	if err != nil {
		// TODO: add logging
		return
	}
	defer c.Close()

	err = r.Write(bc)
	if err != nil {
		// TODO: add logging
		return
	}

	errc := make(chan error, 2)
	cp := func(dst io.Writer, src io.Reader) {
		_, err := io.Copy(dst, src)
		errc <- err
	}
	go cp(bc, c)
	go cp(c, bc)
	<-errc
}

func main() {
	l := listener()
	err := http.Serve(l, handler{
		"loe.yt":       dispatcher,
		"drone.loe.yt": droneWs,
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
	tlsListener = true
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
