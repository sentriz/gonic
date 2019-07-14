package ctrlsubsonic

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"log"
	"net/http"

	"senan.xyz/g/gonic/server/ctrlbase"
	"senan.xyz/g/gonic/server/ctrlsubsonic/spec"
	"senan.xyz/g/gonic/server/parsing"
)

type Controller struct {
	*ctrlbase.Controller
}

func New(base *ctrlbase.Controller) *Controller {
	return &Controller{
		Controller: base,
	}
}

type metaResponse struct {
	XMLName        xml.Name `xml:"subsonic-response" json:"-"`
	*spec.Response `json:"subsonic-response"`
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

func writeResp(w http.ResponseWriter, r *http.Request, resp *spec.Response) {
	res := metaResponse{Response: resp}
	ew := &errWriter{w: w}
	switch parsing.GetStrParam(r, "f") {
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
		ew.write([]byte(parsing.GetStrParamOr(r, "callback", "cb")))
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

type subsonicHandler func(r *http.Request) *spec.Response

func (c *Controller) H(h subsonicHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: write a non 200 if has err
		response := h(r)
		writeResp(w, r, response)
	})
}

type subsonicHandlerRaw func(w http.ResponseWriter, r *http.Request) *spec.Response

func (c *Controller) HR(h subsonicHandlerRaw) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: write a non 200 if has err
		// TODO: ensure no mixed return/writer
		response := h(w, r)
		writeResp(w, r, response)
	})
}
