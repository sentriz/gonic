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

func sessLogSave(w http.ResponseWriter, r *http.Request, s *sessions.Session) {
	if err := s.Save(r, w); err != nil {
		log.Printf("error saving session: %v\n", err)
	}
}

type Flash struct {
	Message string
	Type    string
}

func sessAddFlashW(message string, s *sessions.Session) {
	s.AddFlash(Flash{
		Message: message,
		Type:    "warning",
	})
}

func sessAddFlashN(message string, s *sessions.Session) {
	s.AddFlash(Flash{
		Message: message,
		Type:    "normal",
	})
}
