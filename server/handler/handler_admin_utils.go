package handler

import (
	"log"
	"net/http"

	"github.com/gorilla/sessions"
)

func firstExisting(or string, strings ...string) string {
	for _, s := range strings {
		if s != "" {
			return s
		}
	}
	return or
}

func sessionLogSave(w http.ResponseWriter, r *http.Request, s *sessions.Session) {
	if err := s.Save(r, w); err != nil {
		log.Printf("error saving session: %v\n", err)
	}
}
