package rootdown

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Router is an http request router. See Add for details.
type Router struct {
	head *segment
}

type segment struct {
	parent   *segment
	children map[string]*segment
	methods  map[string]http.Handler
}

// Add adds a route to the Router. Optional middleware is wrapped around the handler at Add time.
//
// Paths are matched without regard to the presence or absence of trailing slashes.
// If a path contains a wildcard (*), any string may be present in that path segment.
// If a request path cannot be matched, the Router looks for the closest parent route that has a 404 path added and routes to that handler.
func (rr *Router) Add(method, path string, h http.HandlerFunc, middlewares ...Middleware) {
	if rr.head == nil {
		rr.head = &segment{
			children: make(map[string]*segment),
			methods:  make(map[string]http.Handler),
		}
	}
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	seg := rr.head
	for {
		before, after, found := cut(path, "/")
		newseg := seg.children[before]
		if newseg == nil {
			newseg = &segment{
				parent:   seg,
				children: make(map[string]*segment),
				methods:  make(map[string]http.Handler),
			}
			seg.children[before] = newseg
		}
		seg = newseg
		if !found {
			break
		}
		path = after
	}
	var handler http.Handler = h
	for i := len(middlewares) - 1; i >= 0; i-- {
		m := middlewares[i]
		handler = m(handler)
	}
	seg.methods[method] = handler
}

// ServeHTTP fulfills the http.Handler interface.
func (rr *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	path = strings.TrimSuffix(path, "/")
	seg := rr.head
	did404 := false
	for seg != nil {
		before, after, found := cut(path, "/")
		newseg := seg.children[before]
		if newseg == nil {
			newseg = seg.children["*"]
		}
		if newseg == nil {
			did404 = true
			break
		}
		seg = newseg
		if !found {
			break
		}
		path = after
	}
	if did404 || len(seg.methods) == 0 {
		for seg != nil {
			newseg := seg.children["404"]
			if newseg != nil {
				seg = newseg
				break
			}
			seg = seg.parent
		}
	}
	if seg == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	h := seg.methods[r.Method]
	if h == nil {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	h.ServeHTTP(w, r)
}

// Get gets a path segment from a request path by looking for wildcards (*) in the path
// and assigning the corresponding request path segement to the argument pointer.
// Arguments may be pointers to strings, []byte, or int. Byte slice pointer arguments are
// Base-64 decoded. If the path cannot be matched or there is an error decoding a byte slice
// or int path, Get returns false.
func Get(r *http.Request, path string, args ...interface{}) (ok bool) {
	if strings.Count(path, "*") != len(args) {
		panic(fmt.Sprintf("bad path: %q", path))
	}
	rpath := r.URL.Path
	n := 0
	for path != "" {
		path = strings.TrimPrefix(path, "/")
		rpath = strings.TrimPrefix(rpath, "/")
		var prefix, star string
		var found bool
		prefix, path, found = cut(path, "*")
		if !strings.HasPrefix(rpath, prefix) {
			return false
		}
		rpath = strings.TrimPrefix(rpath, prefix)
		if !found {
			break
		}
		star, rpath, found = cut(rpath, "/")
		if sp, ok := args[n].(*string); ok {
			*sp = star
		} else if bp, ok := args[n].(*[]byte); ok {
			b, err := base64.StdEncoding.DecodeString(star)
			if err != nil {
				return false
			}
			*bp = b
		} else {
			bitsize := 0
			switch args[n].(type) {
			case *int:
			case *int32:
				bitsize = 32
			case *int64:
				bitsize = 64
			default:
				panic("unsupported type")
			}
			i, err := strconv.ParseInt(star, 10, bitsize)
			if err != nil {
				return false
			}
			switch ip := args[n].(type) {
			case *int:
				*ip = int(i)
			case *int32:
				*ip = int32(i)
			case *int64:
				*ip = i
			}
		}
		if !found {
			break
		}
		n++
	}

	return rpath == ""
}
