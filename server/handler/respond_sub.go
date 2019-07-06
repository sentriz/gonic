package handler

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"

	"senan.xyz/g/gonic/server/subsonic"
)

type metaResponse struct {
	XMLName            xml.Name `xml:"subsonic-response" json:"-"`
	*subsonic.Response `json:"subsonic-response"`
}

type errWriter struct {
	w   io.Writer
	err error
}

func (ew *errWriter) write(buf []byte) {
	if ew.err != nil {
		return
	}
	_, ew.err = ew.w.Write(buf)
}

func respondRaw(w http.ResponseWriter, r *http.Request,
	code int, sub *subsonic.Response) {
	w.WriteHeader(code)
	res := metaResponse{
		Response: sub,
	}
	ew := &errWriter{w: w}
	switch getStrParam(r, "f") {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		data, err := json.Marshal(res)
		if err != nil {
			log.Printf("could not marshall to json: %v\n", err)
			return
		}
		ew.write(data)
	case "jsonp":
		w.Header().Set("Content-Type", "application/javascript")
		data, err := json.Marshal(res)
		if err != nil {
			log.Printf("could not marshall to json: %v\n", err)
			return
		}
		ew.write([]byte(getStrParamOr(r, "callback", "cb")))
		ew.write([]byte("("))
		ew.write(data)
		ew.write([]byte(");"))
	default:
		w.Header().Set("Content-Type", "application/xml")
		data, err := xml.MarshalIndent(res, "", "    ")
		if err != nil {
			log.Printf("could not marshall to xml: %v\n", err)
			return
		}
		ew.write(data)
	}
	if ew.err != nil {
		log.Printf("error writing to response: %v\n", ew.err)
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
