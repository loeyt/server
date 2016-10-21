package main

import (
	"fmt"
	"net/http"
	"os"

	"loe.yt/server"
	"loe.yt/server/goget"
)

var (
	serverImport = &goget.Import{
		Prefix: "server",
		Vcs:    "git",
		Repo:   "https://github.com/loeyt/server",
	}
)

func main() {
	fmt.Println("starting on :8080")
	err := http.ListenAndServe(":8080", &server.Handler{
		Services: []server.Service{
			goget.NewService(goget.Static{
				"server":                  serverImport,
				"server/cmd":              serverImport,
				"server/cmd/loeyt-server": serverImport,
			}),
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
}
