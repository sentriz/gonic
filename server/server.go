package server

import (
	"net/http"
	"time"

	"senan.xyz/g/gonic/db"
	"senan.xyz/g/gonic/server/handler"
)

type Server struct {
	mux *http.ServeMux
	*handler.Controller
	*http.Server
}

func New(
	db *db.DB,
	musicPath string,
	listenAddr string,
) *Server {
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
	controller := &handler.Controller{
		DB:        db,
		MusicPath: musicPath,
	}
	return &Server{
		mux:        mux,
		Server:     server,
		Controller: controller,
	}
}

type middleware func(next http.HandlerFunc) http.HandlerFunc

func newChain(wares ...middleware) middleware {
	return func(final http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			last := final
			for i := len(wares) - 1; i >= 0; i-- {
				last = wares[i](last)
			}
			last(w, r)
		}
	}
}
