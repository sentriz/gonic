package handler

import (
	"log"
	"net/http"
)

//nolint:interfacer
func (c *Controller) WithLogging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("connection from `%s` for `%s`", r.RemoteAddr, r.URL)
		next.ServeHTTP(w, r)
	}
}
