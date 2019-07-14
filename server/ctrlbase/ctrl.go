package ctrlbase

import (
	"log"
	"net/http"

	"senan.xyz/g/gonic/db"
)

type Controller struct {
	DB        *db.DB
	MusicPath string
}

func (c *Controller) WithLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("connection from `%s` for `%s`", r.RemoteAddr, r.URL)
		next.ServeHTTP(w, r)
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
