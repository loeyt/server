package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
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

	handler = &server.Handler{
		Services: []server.Service{
			server.Redirect("https://luit.eu/", http.StatusFound,
				"/",
			),
			goget.NewService(goget.Static{
				"server":                  serverImport,
				"server/cmd":              serverImport,
				"server/cmd/loeyt-server": serverImport,
			}),
		},
	}
)

func main() {
	l := listener()
	err := http.Serve(l, handler)
	if err != nil {
		log.Fatalln(err)
	}
}

func listener() net.Listener {
	files := filesWithNames()
	if files == nil {
		l, err := net.Listen("tcp", ":8080")
		if err != nil {
			log.Fatal(err)
		}
		return l
	}
	var rv net.Listener
	for name, file := range files {
		switch name {
		case "http":
			serveHTTP(file)
		case "https":
			l, err := net.FileListener(file)
			if err != nil {
				log.Fatalln("error using fd as net.Listener:", err)
			}
			rv = tls.NewListener(l, tlsConfig())
		default:
			log.Fatalln("unexpected fd:", name)
		}
	}
	return rv
}

func serveHTTP(f *os.File) {
	l, err := net.FileListener(f)
	if err != nil {
		log.Fatalln("error using fd as net.Listener:", err)
	}
	go http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://"+r.Host+r.URL.Path, http.StatusPermanentRedirect)
	}))
}

func filesWithNames() map[string]*os.File {
	rv := map[string]*os.File{}
	names := strings.Split(os.Getenv("LISTEN_FDNAMES"), ":")
	nfds, err := strconv.Atoi(os.Getenv("LISTEN_FDS"))
	if err != nil {
		return nil
	}
	if len(names) != nfds {
		return nil
	}
	for i, name := range names {
		rv[name] = os.NewFile(uintptr(i+3), "socket_activation_"+name)
	}
	return rv
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
