package goget_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"loe.yt/server"
	"loe.yt/server/goget"
)

var serverImport = &goget.Import{
	Prefix:   "server",
	Vcs:      "git",
	Repo:     "https://github.com/loeyt/server",
	Redirect: "https://loe.yt/some/redirect/target",
}

func TestStatic(t *testing.T) {
	s := httptest.NewServer(&server.Handler{
		Services: []server.Service{
			goget.NewService(goget.Static{
				"server":                  serverImport,
				"server/cmd":              serverImport,
				"server/cmd/loeyt-server": serverImport,
			}),
		},
	})
	defer s.Close()
	host := s.Listener.Addr().String()
	expect := host + "/server git https://github.com/loeyt/server"
	for _, test := range []string{
		host + "/server",
		host + "/server/cmd",
		host + "/server/cmd/loeyt-server",
	} {
		got, err := getGoImport(test)
		if err != nil {
			t.Fatal("error in getGoImport:", err)
		}
		if got != expect {
			t.Fatalf("%s: expected %q, got %q", test, expect, got)
		}
	}
}

func getGoImport(path string) (string, error) {
	resp, err := http.Get("http://" + path + "?go-get=1")
	if err != nil {
		return "", err
	}
	b := &bytes.Buffer{}
	_, err = io.Copy(b, resp.Body)
	if err != nil {
		return "", err
	}
	err = resp.Body.Close()
	if err != nil {
		return "", err
	}
	c := strings.TrimPrefix(b.String(), `<meta name="go-import" content="`)
	c = strings.TrimSuffix(c, "\">\n")
	return c, nil
}
