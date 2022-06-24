package rootdown_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/carlmjohnson/be"
	"github.com/carlmjohnson/requests"
	"github.com/carlmjohnson/rootdown"
)

func TestRouter(t *testing.T) {
	text := func(s string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, s)
		}
	}
	fsys1 := fstest.MapFS{
		"not-found": &fstest.MapFile{
			Data: []byte(`I should not be found`),
		},
		"mount/1.txt": &fstest.MapFile{
			Data: []byte(`1`),
		},
		"mount/1/index.html": &fstest.MapFile{
			Data: []byte(`index`),
		},
		"mount/1/2.txt": &fstest.MapFile{
			Data: []byte(`2`),
		},
	}
	fsys2 := fstest.MapFS{
		"3.txt": &fstest.MapFile{
			Data: []byte(`3`),
		},
		"4/index.html": &fstest.MapFile{
			Data: []byte(`4`),
		},
	}
	var rr rootdown.Router
	rr.Get("/", text("home"))
	rr.Get("/a", text("a"), rootdown.RedirectToSlash)
	rr.Post("/a", text("post"), rootdown.RedirectFromSlash)
	rr.Get("/*/b", text("b"))
	rr.Get("/a/b/c", text("c"))
	rr.Get("/a/b/...", text("404-2"))
	rr.Mount("/static", "mount", fsys1)
	rr.Mount("", "", fsys2)
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
		{http.MethodGet, "/not-found", "404"},
		{http.MethodGet, "/static/not-found", "404"},
		{http.MethodGet, "/static/1.txt", "1"},
		{http.MethodPost, "/static/1.txt", "Method Not Allowed\n"},
		{http.MethodGet, "/static/1/", "index"},
		{http.MethodGet, "/static/1/2.txt", "2"},
		{http.MethodGet, "/3.txt", "3"},
		{http.MethodGet, "/4/", "4"},
	} {
		t.Run(o.method+" "+o.path, func(t *testing.T) {
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
			be.NilErr(t, err)
			be.DebugLog(t, "%s %s: %q", o.method, o.path, s)
			be.DebugLog(t, "%#v\n\n", rr)
			be.Equal(t, o.expect, s)
		})
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
		be.NilErr(t, err)

		got := want
		got.ok = rootdown.Get(r, want.pat, &got.s)
		be.Equal(t, want, got)
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
		be.NilErr(t, err)

		got := want
		got.ok = rootdown.Get(r, want.pat, &got.i)
		be.Equal(t, want, got)

		var i32 int32
		got.ok = rootdown.Get(r, want.pat, &i32)
		got.i = int(i32)
		be.Equal(t, want, got)

		var i64 int64
		got.ok = rootdown.Get(r, want.pat, &i64)
		got.i = int(i64)
		be.Equal(t, want, got)
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
		be.NilErr(t, err)

		got := want
		var b []byte
		got.ok = rootdown.Get(r, want.pat, &b)
		got.s = string(b)
		be.Equal(t, want, got)
	}
}

func BenchmarkRouterServeHTTP(b *testing.B) {
	text := func(s string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, s)
		}
	}
	var rr rootdown.Router
	rr.Get("/", text("home"))
	rr.Get("/a/b/c", text("c"))
	rr.NotFound(text("404"))
	req := httptest.NewRequest(http.MethodGet, "/a/b/c", nil)
	w := &httptest.ResponseRecorder{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		*w = httptest.ResponseRecorder{}
		rr.ServeHTTP(w, req)
	}
}
