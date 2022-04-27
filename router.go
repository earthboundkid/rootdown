// Package rootdown is the only HTTP router for Go.
package rootdown

import (
	"encoding/base64"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	pkgpath "path"
	"strconv"
	"strings"
	"time"
)

// Router is an HTTP request router. See rr.Route for details on routing.
type Router struct {
	head *segment
}

type segment struct {
	parent   *segment
	children map[string]*segment
	methods  map[string]http.Handler
}

// Route adds a route to the Router. Optional middleware is wrapped around the handler at add time.
//
// Methods are case sensitive and should be uppercase. A wildcard (*) will match any method.
//
// Paths are matched without regard to the presence or absence of trailing slashes.
// (See the redirect middleware to enforce the presence/absence of a slash.)
// If a path contains a wildcard (*), any string may be present in that path segment.
// If a request path cannot be matched, the Router looks for the closest parent route that has a 404 path added and routes to that handler.
func (rr *Router) Route(method, path string, h http.HandlerFunc, middlewares ...Middleware) {
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
		before, after, found := strings.Cut(path, "/")
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
	handler := MiddlewareStack(middlewares).AsMiddleware()(h)
	seg.methods[method] = handler
}

// Get is a shortcut for rr.Route(http.MethodGet, ...).
func (rr *Router) Get(path string, h http.HandlerFunc, middlewares ...Middleware) {
	rr.Route(http.MethodGet, path, h, middlewares...)
}

// Post is a shortcut for rr.Route(http.MethodPost, ...).
func (rr *Router) Post(path string, h http.HandlerFunc, middlewares ...Middleware) {
	rr.Route(http.MethodPost, path, h, middlewares...)
}

// NotFound is a shortcut for rr.Route("*", "/404", ...).
func (rr *Router) NotFound(h http.HandlerFunc, middlewares ...Middleware) {
	rr.Route("*", "/404", h, middlewares...)
}

// Mount mounts fsys at the given path by walking the filesystem starting at root and
// adding a Get entry for every file it finds. As a result, if new files are added to
// fsys at runtime, they won't be picked up.
// Mount panics if there are any errors in the process of walking the fsys.
func (rr *Router) Mount(path, root string, fsys fs.FS, middlewares ...Middleware) {
	httpfs := http.FS(fsys)
	if root == "" {
		root = "."
	}
	err := fs.WalkDir(fsys, root, func(fpath string, de fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("could not mount filesystem: %w", err)
		}
		if !de.Type().IsRegular() {
			return nil
		}
		routepath := pkgpath.Join(path, strings.TrimPrefix(fpath, root))
		if de.Name() == "index.html" {
			routepath = strings.TrimSuffix(routepath, "/index.html")
		}
		h := func(w http.ResponseWriter, r *http.Request) {
			f, err := httpfs.Open(fpath)
			if err != nil {
				log.Printf("problem opening mounted file: %v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			defer f.Close()

			var modtime time.Time
			if stat, err := f.Stat(); err == nil {
				modtime = stat.ModTime()
			}

			http.ServeContent(w, r, de.Name(), modtime, f)
		}
		rr.Get(routepath, h, middlewares...)
		return nil
	})
	if err != nil {
		panic(err)
	}
}

// ServeHTTP fulfills the http.Handler interface.
func (rr *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	path = strings.TrimSuffix(path, "/")
	seg := rr.head
	did404 := false
	for seg != nil {
		before, after, found := strings.Cut(path, "/")
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
		h = seg.methods["*"]
	}
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
		prefix, path, found = strings.Cut(path, "*")
		if !strings.HasPrefix(rpath, prefix) {
			return false
		}
		rpath = strings.TrimPrefix(rpath, prefix)
		if !found {
			break
		}
		star, rpath, found = strings.Cut(rpath, "/")
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
