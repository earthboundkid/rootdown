package rootdown

import (
	"net/http"
	"strings"
)

// Middleware is any function that wraps an http.Handler returning a new http.Handler.
type Middleware = func(h http.Handler) http.Handler

// RedirectToSlash returns a permanent redirect when a request path does not have a trailing slash.
var RedirectToSlash Middleware = redirectToSlash

func redirectToSlash(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// RedirectToSlash returns a permanent redirect when a request path has a trailing slash.
var RedirectFromSlash Middleware = redirectFromSlash

func redirectFromSlash(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, "/"), http.StatusMovedPermanently)
			return
		}
		h.ServeHTTP(w, r)
	})
}
