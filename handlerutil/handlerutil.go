package handlerutil

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

type Middleware func(http.Handler) http.Handler

func Chain(middlewares ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

func TrimPathSuffix(suffix string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = strings.TrimSuffix(r.URL.Path, suffix)
			next.ServeHTTP(w, r)
		})
	}
}

func Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		log.Printf("response %s %s %v", statusToBlock(sw.status), r.Method, r.URL)
	})
}

func BasicCORS(next http.Handler) http.Handler {
	allowMethods := strings.Join(
		[]string{http.MethodPost, http.MethodGet, http.MethodOptions, http.MethodPut, http.MethodDelete},
		", ",
	)
	allowHeaders := strings.Join(
		[]string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"},
		", ",
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", allowMethods)
		w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
		if r.Method == http.MethodOptions {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func Redirect(to string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, to, http.StatusSeeOther)
	})
}

func Message(message string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, message)
	})
}

func BaseURL(r *http.Request) string {
	fallbackProtocoll := "http"
	if r.TLS != nil {
		fallbackProtocoll = "https"
	}
	fallbackHost := "localhost:4747"
	scheme := first(
		r.Header.Get("X-Forwarded-Proto"),
		r.Header.Get("X-Forwarded-Scheme"),
		r.URL.Scheme,
		fallbackProtocoll,
	)
	host := first(
		r.Header.Get("X-Forwarded-Host"),
		r.Host,
		fallbackHost,
	)
	return fmt.Sprintf("%s://%s", scheme, host)
}

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
	case code >= 500:
		bg = 41 // bright red
	case code >= 400:
		bg = 43 // bright orange
	case code >= 300:
		bg = 46 // bright cyan
	case code >= 200:
		bg = 42 // bright green
	default:
		bg = 47 // bright white (grey)
	}
	return fmt.Sprintf("\u001b[%d;1m %d \u001b[0m", bg, code)
}

func first[T comparable](vs ...T) T {
	var z T
	for _, s := range vs {
		if s != z {
			return s
		}
	}
	return z
}
