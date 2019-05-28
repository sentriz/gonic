package handler

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"

	"github.com/sentriz/gonic/server/subsonic"
)

func respondRaw(w http.ResponseWriter, r *http.Request,
	code int, sub *subsonic.Response) {
	res := subsonic.MetaResponse{
		Response: sub,
	}
	switch r.URL.Query().Get("f") {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		data, err := json.Marshal(res)
		if err != nil {
			log.Printf("could not marshall to json: %v\n", err)
		}
		w.Write(data)
	case "jsonp":
		w.Header().Set("Content-Type", "application/javascript")
		data, err := json.Marshal(res)
		if err != nil {
			log.Printf("could not marshall to json: %v\n", err)
		}
		callback := r.URL.Query().Get("callback")
		w.Write([]byte(callback))
		w.Write([]byte("("))
		w.Write(data)
		w.Write([]byte(");"))
	default:
		w.Header().Set("Content-Type", "application/xml")
		data, err := xml.MarshalIndent(res, "", "    ")
		if err != nil {
			log.Printf("could not marshall to xml: %v\n", err)
		}
		w.Write(data)
	}
}

func respond(w http.ResponseWriter, r *http.Request,
	sub *subsonic.Response) {
	respondRaw(w, r, http.StatusOK, sub)
}

func respondError(w http.ResponseWriter, r *http.Request,
	code int, message string, a ...interface{}) {
	respondRaw(w, r, http.StatusBadRequest, subsonic.NewError(
		code, fmt.Sprintf(message, a...),
	))
}
