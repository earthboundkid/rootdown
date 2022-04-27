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

// RedirectFromSlash returns a permanent redirect when a request path has a trailing slash.
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

// MiddlewareStack is a slice of Middleware for use with Router.
type MiddlewareStack []Middleware

// Push adds a Middleware to end of the stack.
func (stack *MiddlewareStack) Push(mw Middleware) {
	*stack = append(*stack, mw)
}

// Clone returns a shallow copy of the stack.
func (stack MiddlewareStack) Clone() MiddlewareStack {
	clone := make(MiddlewareStack, len(stack))
	copy(clone, stack)
	return clone
}

// As Middleware returns a Middleware which applies each of the members of the stack to its handlers.
func (stack MiddlewareStack) AsMiddleware() Middleware {
	return func(h http.Handler) http.Handler {
		for i := len(stack) - 1; i >= 0; i-- {
			m := (stack)[i]
			h = m(h)
		}
		return h
	}
}
