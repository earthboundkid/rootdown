package rootdown_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/carlmjohnson/rootdown"
)

func ExampleMiddlewareStack() {
	mw1 := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("middleware 1 start")
			h.ServeHTTP(w, r)
			fmt.Println("middleware 1 end")
		})
	}
	mw2 := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("middleware 2 start")
			h.ServeHTTP(w, r)
			fmt.Println("middleware 2 end")
		})
	}

	h := func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("running handler")
		fmt.Fprintln(w, "hello, world")
	}

	var middlewares rootdown.MiddlewareStack
	middlewares.Push(mw1)
	middlewares.Push(mw2)

	var rr rootdown.Router
	rr.Get("/", h, middlewares...)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr.ServeHTTP(&httptest.ResponseRecorder{}, req)
	// Output:
	// middleware 1 start
	// middleware 2 start
	// running handler
	// middleware 2 end
	// middleware 1 end
}
