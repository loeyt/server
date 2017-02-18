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
	"time"

	"loe.yt/server"
	"loe.yt/server/goget"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/context"
)

const leStagingURL = "https://acme-staging.api.letsencrypt.org/directory"

// Version is the loeyt-server version, changed by make when built by CI.
var Version = "dev"

// BuildTime is the loeyt-server build time, changed by make when built by CI.
var BuildTime = "unknown"

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
			versionHandler{},
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
	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler: handler{
			"loe.yt":       dispatcher,
			"drone.loe.yt": droneWs,
		},
	}
	err := srv.Serve(l)
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
	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://"+r.Host+r.URL.Path, http.StatusPermanentRedirect)
		}),
	}
	go srv.Serve(l)
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
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.X25519,
		},
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
		NextProtos: []string{"h2", "http/1.1"},
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

type versionHandler struct{}

func (h versionHandler) MatchHTTP(r *http.Request) (bool, error) {
	if r.URL.Path == "/_debug/version" {
		return true, nil
	}
	return false, nil
}

func (h versionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "version: %s\nbuilt: %s", Version, BuildTime)
}
