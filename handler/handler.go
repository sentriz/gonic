package handler

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"

	"github.com/jinzhu/gorm"
	"github.com/sentriz/gonic/subsonic"
)

type Controller struct {
	DB *gorm.DB
}

func respondRaw(w http.ResponseWriter, r *http.Request, code int, sub *subsonic.Response) {
	format := r.URL.Query().Get("f")
	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		data, err := json.Marshal(sub)
		if err != nil {
			log.Printf("could not marshall to json: %v\n", err)
		}
		w.Write([]byte(`{"subsonic-response":`))
		w.Write(data)
		w.Write([]byte("}"))
	case "jsonp":
		w.Header().Set("Content-Type", "application/javascript")
		data, err := json.Marshal(sub)
		if err != nil {
			log.Printf("could not marshall to json: %v\n", err)
		}
		callback := r.URL.Query().Get("callback")
		w.Write([]byte(fmt.Sprintf(`%s({"subsonic-response":`, callback)))
		w.Write(data)
		w.Write([]byte("});"))
	default:
		w.Header().Set("Content-Type", "application/xml")
		data, err := xml.Marshal(sub)
		if err != nil {
			log.Printf("could not marshall to xml: %v\n", err)
		}
		w.Write(data)
	}
}

func respond(w http.ResponseWriter, r *http.Request, sub *subsonic.Response) {
	respondRaw(w, r, http.StatusOK, sub)
}

func respondError(w http.ResponseWriter, r *http.Request, code uint64, message string) {
	respondRaw(w, r, http.StatusBadRequest, subsonic.NewError(
		code, message,
	))
}
