package handler

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"

	"github.com/sentriz/gonic/server/subsonic"
)

type metaResponse struct {
	XMLName            xml.Name `xml:"subsonic-response" json:"-"`
	*subsonic.Response `json:"subsonic-response"`
}

func respondRaw(w http.ResponseWriter, r *http.Request,
	code int, sub *subsonic.Response) {
	w.WriteHeader(code)
	res := metaResponse{
		Response: sub,
	}
	switch getStrParam(r, "f") {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		data, err := json.Marshal(res)
		if err != nil {
			log.Printf("could not marshall to json: %v\n", err)
			return
		}
		w.Write(data)
	case "jsonp":
		w.Header().Set("Content-Type", "application/javascript")
		data, err := json.Marshal(res)
		if err != nil {
			log.Printf("could not marshall to json: %v\n", err)
			return
		}
		w.Write([]byte(getStrParamOr(r, "callback", "cb")))
		w.Write([]byte("("))
		w.Write(data)
		w.Write([]byte(");"))
	default:
		w.Header().Set("Content-Type", "application/xml")
		data, err := xml.MarshalIndent(res, "", "    ")
		if err != nil {
			log.Printf("could not marshall to xml: %v\n", err)
			return
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
