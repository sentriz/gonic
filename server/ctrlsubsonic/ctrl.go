package ctrlsubsonic

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"log"
	"net/http"

	"github.com/pkg/errors"

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

func writeResp(w http.ResponseWriter, r *http.Request, resp *spec.Response) error {
	if resp.Error != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
	res := metaResponse{Response: resp}
	ew := &errWriter{w: w}
	switch parsing.GetStrParam(r, "f") {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		data, err := json.Marshal(res)
		if err != nil {
			return errors.Wrap(err, "marshal to json")
		}
		ew.write(data)
	case "jsonp":
		w.Header().Set("Content-Type", "application/javascript")
		data, err := json.Marshal(res)
		if err != nil {
			return errors.Wrap(err, "marshal to jsonp")
		}
		// TODO: error if no callback provided instead of using a default
		pCall := parsing.GetStrParamOr(r, "callback", "cb")
		ew.write([]byte(pCall))
		ew.write([]byte("("))
		ew.write(data)
		ew.write([]byte(");"))
	default:
		w.Header().Set("Content-Type", "application/xml")
		data, err := xml.MarshalIndent(res, "", "    ")
		if err != nil {
			return errors.Wrap(err, "marshal to xml")
		}
		ew.write(data)
	}
	return ew.err
}

type subsonicHandler func(r *http.Request) *spec.Response

func (c *Controller) H(h subsonicHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := h(r)
		if response == nil {
			log.Println("error: non raw subsonic handler returned a nil response")
			return
		}
		if err := writeResp(w, r, response); err != nil {
			log.Printf("error writing subsonic response (normal handler): %v\n", err)
		}
	})
}

type subsonicHandlerRaw func(w http.ResponseWriter, r *http.Request) *spec.Response

func (c *Controller) HR(h subsonicHandlerRaw) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := h(w, r)
		if response == nil {
			return
		}
		if err := writeResp(w, r, response); err != nil {
			log.Printf("error writing subsonic response (raw handler): %v\n", err)
		}
	})
}
