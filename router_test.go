package rootdown_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/carlmjohnson/requests"
	"github.com/carlmjohnson/rootdown"
)

func TestRouter(t *testing.T) {
	text := func(s string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, s)
		}
	}
	var rr rootdown.Router
	rr.Add(http.MethodGet, "/", text("home"))
	rr.Add(http.MethodGet, "/a", text("a"), rootdown.RedirectToSlash)
	rr.Add(http.MethodPost, "/a", text("post"), rootdown.RedirectFromSlash)
	rr.Add(http.MethodGet, "/*/b", text("b"))
	rr.Add(http.MethodGet, "/a/b/c", text("c"))
	rr.Add(http.MethodGet, "/404", text("404"))
	srv := httptest.NewServer(&rr)
	for _, o := range []struct{ method, path, expect string }{
		{http.MethodGet, "/", "home"},
		{http.MethodGet, "/a", "<a href=\"/a/\">Moved Permanently</a>.\n\n"},
		{http.MethodGet, "/a/", "a"},
		{http.MethodGet, "/a//", "404"},
		{http.MethodGet, "/bleh/b", "b"},
		{http.MethodGet, "/a/b/c", "c"},
		{http.MethodGet, "/xxx", "404"},
		{http.MethodGet, "/xxx/yyy", "404"},
		{http.MethodGet, "/a/yyy", "404"},
		{http.MethodPost, "/a/", ""},
		{http.MethodPost, "/a", "post"},
		{http.MethodPost, "/bleh/b", "Method Not Allowed\n"},
	} {
		var s string
		cl := srv.Client()
		cl.CheckRedirect = requests.NoFollow
		err := requests.
			URL(srv.URL + o.path).
			Method(o.method).
			Client(cl).
			AddValidator(nil).
			ToString(&s).
			Fetch(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if s != o.expect {
			t.Errorf("%s %s: %q", o.method, o.path, s)
		}
	}
}

func TestParam(t *testing.T) {
	type test struct {
		req, pat, s string
		ok          bool
	}
	for _, want := range []test{
		{"http://x.com/a", "/*", "a", true},
		{"http://x.com/a/", "/*/", "a", true},
		{"http://x.com/a/b", "/*/b", "a", true},
		{"http://x.com/a/b", "/*/c", "a", false},
		{"http://x.com/a/b", "/a/*", "b", true},
		{"http://x.com/a/b/c", "/*/b/c", "a", true},
		{"http://x.com/a/b/c", "/*/b/d", "a", false},
		{"http://x.com/a/b/c", "/a/b/*", "c", true},
		{"http://x.com/a/b/c", "/a/x/*", "", false},
	} {
		r, err := http.NewRequest(http.MethodGet, want.req, nil)
		if err != nil {
			t.Fatal(err)
		}

		got := want
		got.ok = rootdown.Param(r, want.pat, &got.s)
		if want != got {
			t.Fatalf("want %#v; got %#v", want, got)
		}
	}
}
