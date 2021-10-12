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
	rr.Get("/", text("home"))
	rr.Get("/a", text("a"), rootdown.RedirectToSlash)
	rr.Post("/a", text("post"), rootdown.RedirectFromSlash)
	rr.Get("/*/b", text("b"))
	rr.Get("/a/b/c", text("c"))
	rr.Get("/a/b/404", text("404-2"))
	rr.NotFound(text("404"))
	srv := httptest.NewServer(&rr)
	for _, o := range []struct{ method, path, expect string }{
		{http.MethodGet, "/", "home"},
		{http.MethodGet, "/a", "<a href=\"/a/\">Moved Permanently</a>.\n\n"},
		{http.MethodGet, "/a/", "a"},
		{http.MethodGet, "/a//", "404"},
		{http.MethodGet, "/bleh/b", "b"},
		{http.MethodGet, "/a/b/c", "c"},
		{http.MethodGet, "/a/b/d", "404-2"},
		{http.MethodGet, "/xxx", "404"},
		{http.MethodGet, "/xxx/yyy", "404"},
		{http.MethodGet, "/a/yyy", "404"},
		{http.MethodPost, "/a/", ""},
		{http.MethodPost, "/a", "post"},
		{http.MethodPost, "/c", "404"},
		{http.MethodPost, "/bleh/b", "Method Not Allowed\n"},
	} {
		var s string
		cl := srv.Client()
		cl.CheckRedirect = requests.NoFollow
		err := requests.
			URL(srv.URL).
			Path(o.path).
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
	type testStr struct {
		req, pat, s string
		ok          bool
	}
	for _, want := range []testStr{
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
		got.ok = rootdown.Get(r, want.pat, &got.s)
		if want != got {
			t.Fatalf("want %#v; got %#v", want, got)
		}
	}
	type testInt struct {
		req, pat string
		i        int
		ok       bool
	}
	for _, want := range []testInt{
		{"http://x.com/1", "/*", 1, true},
		{"http://x.com/1/", "/*/", 1, true},
		{"http://x.com/1/b", "/*/b", 1, true},
		{"http://x.com/1/b", "/*/c", 1, false},
		{"http://x.com/a/1", "/a/*", 1, true},
		{"http://x.com/1/b/c", "/*/b/c", 1, true},
		{"http://x.com/a/b/c", "/*/b/d", 0, false},
		{"http://x.com/a/b/1", "/a/b/*", 1, true},
		{"http://x.com/a/b/1", "/a/x/*", 0, false},
	} {
		r, err := http.NewRequest(http.MethodGet, want.req, nil)
		if err != nil {
			t.Fatal(err)
		}

		got := want
		got.ok = rootdown.Get(r, want.pat, &got.i)
		if want != got {
			t.Fatalf("want %#v; got %#v", want, got)
		}
		var i32 int32
		got.ok = rootdown.Get(r, want.pat, &i32)
		got.i = int(i32)
		if want != got {
			t.Fatalf("want %#v; got %#v", want, got)
		}
		var i64 int64
		got.ok = rootdown.Get(r, want.pat, &i64)
		got.i = int(i64)
		if want != got {
			t.Fatalf("want %#v; got %#v", want, got)
		}
	}
	type testByte struct {
		req, pat, s string
		ok          bool
	}
	for _, want := range []testByte{
		{"http://x.com/YQ==", "/*", "a", true},
		{"http://x.com/YQ==/", "/*/", "a", true},
		{"http://x.com/YQ==/b", "/*/b", "a", true},
		{"http://x.com/YQ==/b", "/*/c", "a", false},
		{"http://x.com/a/Yg==", "/a/*", "b", true},
		{"http://x.com/YQ==/b/c", "/*/b/c", "a", true},
		{"http://x.com/YQ==/b/c", "/*/b/d", "a", false},
		{"http://x.com/a/b/Yw==", "/a/b/*", "c", true},
		{"http://x.com/a/b/Yw==", "/a/x/*", "", false},
	} {
		r, err := http.NewRequest(http.MethodGet, want.req, nil)
		if err != nil {
			t.Fatal(err)
		}

		got := want
		var b []byte
		got.ok = rootdown.Get(r, want.pat, &b)
		got.s = string(b)
		if want != got {
			t.Fatalf("want %#v; got %#v", want, got)
		}
	}
}
