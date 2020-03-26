package ctrlsubsonic

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"log"
	"net/http"

	"github.com/pkg/errors"

	"senan.xyz/g/gonic/server/ctrlbase"
	"senan.xyz/g/gonic/server/ctrlsubsonic/params"
	"senan.xyz/g/gonic/server/ctrlsubsonic/spec"
)

type CtxKey int

const (
	CtxUser CtxKey = iota
	CtxSession
	CtxParams
)

type Controller struct {
	*ctrlbase.Controller
	cachePath string
}

func New(base *ctrlbase.Controller, cachePath string) *Controller {
	return &Controller{
		Controller: base,
		cachePath:  cachePath,
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
	res := metaResponse{Response: resp}
	params := r.Context().Value(CtxParams).(params.Params)
	ew := &errWriter{w: w}
	switch params.Get("f") {
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
		pCall := params.GetOr("callback", "cb")
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

type handlerSubsonic func(r *http.Request) *spec.Response

func (c *Controller) H(h handlerSubsonic) http.Handler {
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

type handlerSubsonicRaw func(w http.ResponseWriter, r *http.Request) *spec.Response

func (c *Controller) HR(h handlerSubsonicRaw) http.Handler {
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
