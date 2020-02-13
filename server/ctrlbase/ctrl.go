package ctrlbase

import (
	"fmt"
	"log"
	"net/http"
	"path"

	"senan.xyz/g/gonic/db"
	"senan.xyz/g/gonic/scanner"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.ResponseWriter.Write(b)
}

func statusToBlock(code int) string {
	var bg int
	switch {
	case 200 <= code && code <= 299:
		bg = 42 // bright green, ok
	case 300 <= code && code <= 399:
		bg = 46 // bright cyan, redirect
	case 400 <= code && code <= 499:
		bg = 43 // bright orange, client error
	case 500 <= code && code <= 599:
		bg = 41 // bright red, server error
	default:
		bg = 47 // bright white (grey)
	}
	return fmt.Sprintf("\u001b[%d;1m %d \u001b[0m", bg, code)
}

type Controller struct {
	DB          *db.DB
	MusicPath   string
	CachePath   string
	Scanner     *scanner.Scanner
	ProxyPrefix string
}

// Path returns a URL path with the proxy prefix included
func (c *Controller) Path(rel string) string {
	return path.Join(c.ProxyPrefix, rel)
}

func (c *Controller) WithLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// this is (should be) the first middleware. pass right though it
		// by calling `next` first instead of last. when it completes all
		// other middlewares and the custom ResponseWriter has been written
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		log.Printf("response %s for `%s`", statusToBlock(sw.status), r.URL)
	})
}

func (c *Controller) WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods",
			"POST, GET, OPTIONS, PUT, DELETE",
		)
		w.Header().Set("Access-Control-Allow-Headers",
			"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization",
		)
		if r.Method == "OPTIONS" {
			return
		}
		next.ServeHTTP(w, r)
	})
}
