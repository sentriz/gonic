// Package ctrlsubsonic provides HTTP handlers for subsonic API
package ctrlsubsonic

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"

	"go.senan.xyz/gonic/jukebox"
	"go.senan.xyz/gonic/podcasts"
	"go.senan.xyz/gonic/scrobble"
	"go.senan.xyz/gonic/server/ctrlbase"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/transcode"
)

type CtxKey int

const (
	CtxUser CtxKey = iota
	CtxSession
	CtxParams
)

type MusicPath struct {
	Alias, Path string
}

func PathsOf(paths []MusicPath) []string {
	var r []string
	for _, p := range paths {
		r = append(r, p.Path)
	}
	return r
}

type Controller struct {
	*ctrlbase.Controller
	MusicPaths     []MusicPath
	PodcastsPath   string
	CacheAudioPath string
	CoverCachePath string
	Jukebox        *jukebox.Jukebox
	Scrobblers     []scrobble.Scrobbler
	Podcasts       *podcasts.Podcasts
	Transcoder     transcode.Transcoder
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
	if resp == nil {
		return nil
	}
	if resp.Error != nil {
		log.Printf("subsonic error code %d: %s", resp.Error.Code, resp.Error.Message)
	}

	res := metaResponse{Response: resp}
	params := r.Context().Value(CtxParams).(params.Params)
	ew := &errWriter{w: w}
	switch v, _ := params.Get("f"); v {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		data, err := json.Marshal(res)
		if err != nil {
			return fmt.Errorf("marshal to json: %w", err)
		}
		ew.write(data)
	case "jsonp":
		w.Header().Set("Content-Type", "application/javascript")
		data, err := json.Marshal(res)
		if err != nil {
			return fmt.Errorf("marshal to jsonp: %w", err)
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
			return fmt.Errorf("marshal to xml: %w", err)
		}
		ew.write(data)
	}
	return ew.err
}

type (
	handlerSubsonic    func(r *http.Request) *spec.Response
	handlerSubsonicRaw func(w http.ResponseWriter, r *http.Request) *spec.Response
)

func (c *Controller) H(h handlerSubsonic) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := writeResp(w, r, h(r)); err != nil {
			log.Printf("error writing subsonic response: %v\n", err)
		}
	})
}

func (c *Controller) HR(h handlerSubsonicRaw) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := writeResp(w, r, h(w, r)); err != nil {
			log.Printf("error writing raw subsonic response: %v\n", err)
		}
	})
}
