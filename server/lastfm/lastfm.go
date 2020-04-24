package lastfm

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"go.senan.xyz/gonic/server/db"
)

var (
	baseURL = "https://ws.audioscrobbler.com/2.0/"
	client  = &http.Client{
		Timeout: 10 * time.Second,
	}
)

// TODO: remove this package's dependency on models/db

func getParamSignature(params url.Values, secret string) string {
	// the parameters must be in order before hashing
	paramKeys := make([]string, 0)
	for k := range params {
		paramKeys = append(paramKeys, k)
	}
	sort.Strings(paramKeys)
	toHash := ""
	for _, k := range paramKeys {
		toHash += k
		toHash += params[k][0]
	}
	toHash += secret
	hash := md5.Sum([]byte(toHash))
	return hex.EncodeToString(hash[:])
}

func makeRequest(method string, params url.Values) (LastFM, error) {
	req, _ := http.NewRequest(method, baseURL, nil)
	req.URL.RawQuery = params.Encode()
	resp, err := client.Do(req)
	if err != nil {
		return LastFM{}, errors.Wrap(err, "get")
	}
	defer resp.Body.Close()
	decoder := xml.NewDecoder(resp.Body)
	lastfm := LastFM{}
	if err = decoder.Decode(&lastfm); err != nil {
		return LastFM{}, errors.Wrap(err, "decoding")
	}
	if lastfm.Error.Code != 0 {
		return LastFM{}, fmt.Errorf("parsing: %v", lastfm.Error.Value)
	}
	return lastfm, nil
}

func GetSession(apiKey, secret, token string) (string, error) {
	params := url.Values{}
	params.Add("method", "auth.getSession")
	params.Add("api_key", apiKey)
	params.Add("token", token)
	params.Add("api_sig", getParamSignature(params, secret))
	resp, err := makeRequest("GET", params)
	if err != nil {
		return "", errors.Wrap(err, "making session GET")
	}
	return resp.Session.Key, nil
}

type ScrobbleOpts struct {
	Track      *db.Track
	StampMili  int
	Submission bool
}

func Scrobble(apiKey, secret, session string, opts ScrobbleOpts) error {
	params := url.Values{}
	if opts.Submission {
		params.Add("method", "track.Scrobble")
		// last.fm wants the timestamp in seconds
		params.Add("timestamp", strconv.Itoa(opts.StampMili/1e3))
	} else {
		params.Add("method", "track.updateNowPlaying")
	}
	params.Add("api_key", apiKey)
	params.Add("sk", session)
	params.Add("artist", opts.Track.TagTrackArtist)
	params.Add("track", opts.Track.TagTitle)
	params.Add("trackNumber", strconv.Itoa(opts.Track.TagTrackNumber))
	params.Add("album", opts.Track.Album.TagTitle)
	params.Add("mbid", opts.Track.TagBrainzID)
	params.Add("albumArtist", opts.Track.Artist.Name)
	params.Add("api_sig", getParamSignature(params, secret))
	_, err := makeRequest("POST", params)
	return err
}

func ArtistGetInfo(apiKey string, artist *db.Artist) (Artist, error) {
	params := url.Values{}
	params.Add("method", "artist.getInfo")
	params.Add("api_key", apiKey)
	params.Add("artist", artist.Name)
	resp, err := makeRequest("GET", params)
	if err != nil {
		return Artist{}, errors.Wrap(err, "making artist GET")
	}
	return resp.Artist, nil
}
