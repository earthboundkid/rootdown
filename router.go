package rootdown

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type Router struct {
	head *segment
}

type segment struct {
	parent   *segment
	children map[string]*segment
	methods  map[string]http.Handler
}

func (rr *Router) Add(method, path string, h http.HandlerFunc, middlewares ...func(h http.Handler) http.Handler) {
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

func cut(s, sep string) (before, after string, found bool) {
	if i := strings.Index(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}

func RedirectToSlash(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func RedirectFromSlash(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, "/"), http.StatusMovedPermanently)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func Param(r *http.Request, path string, args ...interface{}) (ok bool) {
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
