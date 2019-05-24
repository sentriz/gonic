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

	"github.com/sentriz/gonic/model"
)

var (
	baseURL = "https://ws.audioscrobbler.com/2.0/"
	client  = &http.Client{
		Timeout: 10 * time.Second,
	}
)

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

func Scrobble(apiKey, secret, session string, track *model.Track,
	stampMili int, submission bool) error {
	params := url.Values{}
	if submission {
		params.Add("method", "track.Scrobble")
		// last.fm wants the timestamp in seconds
		params.Add("timestamp", strconv.Itoa(stampMili/1e3))
	} else {
		params.Add("method", "track.updateNowPlaying")
	}
	params.Add("api_key", apiKey)
	params.Add("sk", session)
	params.Add("artist", track.Artist)
	params.Add("track", track.Title)
	params.Add("album", track.Album.Title)
	params.Add("albumArtist", track.AlbumArtist.Name)
	params.Add("trackNumber", strconv.Itoa(track.TrackNumber))
	params.Add("api_sig", getParamSignature(params, secret))
	_, err := makeRequest("POST", params)
	return err
}

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

func makeRequest(method string, params url.Values) (*LastFM, error) {
	req, _ := http.NewRequest(method, baseURL, nil)
	req.URL.RawQuery = params.Encode()
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "get")
	}
	defer resp.Body.Close()
	decoder := xml.NewDecoder(resp.Body)
	var lastfm LastFM
	err = decoder.Decode(&lastfm)
	if err != nil {
		return nil, errors.Wrap(err, "decoding")
	}
	if lastfm.Error != nil {
		return nil, fmt.Errorf("parsing: %v", lastfm.Error.Value)
	}
	return &lastfm, nil
}
