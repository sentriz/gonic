package lastfm

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"go.senan.xyz/gonic/server/db"
)

const (
	lastfmBaseURL = "https://ws.audioscrobbler.com/2.0/"
	lbBaseURL     = "https://api.listenbrainz.org"
)

var (
	ErrLastFM       = errors.New("last.fm error")
	ErrListenBrainz = errors.New("listenbrainz error")
)

// TODO: remove this package's dependency on models/db

func getParamSignature(params url.Values, secret string) string {
	// the parameters must be in order before hashing
	paramKeys := make([]string, 0, len(params))
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
	req, _ := http.NewRequest(method, lastfmBaseURL, nil)
	req.URL.RawQuery = params.Encode()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return LastFM{}, fmt.Errorf("get: %w", err)
	}
	defer resp.Body.Close()
	decoder := xml.NewDecoder(resp.Body)
	lastfm := LastFM{}
	if err = decoder.Decode(&lastfm); err != nil {
		return LastFM{}, fmt.Errorf("decoding: %w", err)
	}
	if lastfm.Error.Code != 0 {
		return LastFM{}, fmt.Errorf("%v: %w", lastfm.Error.Value, ErrLastFM)
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
		return "", fmt.Errorf("making session GET: %w", err)
	}
	return resp.Session.Key, nil
}

type ScrobbleOptions struct {
	Track      *db.Track
	StampMili  int
	Submission bool
}

type LastfmScrobbler struct { //nolint
	DB *db.DB
}

func (lfm *LastfmScrobbler) Scrobble(user *db.User, opts ScrobbleOptions) error {
	apiKey := lfm.DB.GetSetting("lastfm_api_key")
	secret := lfm.DB.GetSetting("lastfm_secret")
	// fetch user to get lastfm session
	if user.LastFMSession == "" {
		return fmt.Errorf("you don't have a last.fm session: %w", ErrLastFM)
	}
	params := url.Values{}
	if opts.Submission {
		params.Add("method", "track.Scrobble")
		// last.fm wants the timestamp in seconds
		params.Add("timestamp", strconv.Itoa(opts.StampMili/1e3))
	} else {
		params.Add("method", "track.updateNowPlaying")
	}
	params.Add("api_key", apiKey)
	params.Add("sk", user.LastFMSession)
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

func (lfm *LastfmScrobbler) Enabled(user *db.User) bool {
	return user.LastFMSession != ""
}

func ArtistGetInfo(apiKey string, artist *db.Artist) (Artist, error) {
	params := url.Values{}
	params.Add("method", "artist.getInfo")
	params.Add("api_key", apiKey)
	params.Add("artist", artist.Name)
	resp, err := makeRequest("GET", params)
	if err != nil {
		return Artist{}, fmt.Errorf("making artist GET: %w", err)
	}
	return resp.Artist, nil
}

type ListenBrainzScrobbler struct {
	DB *db.DB
}

func (lb *ListenBrainzScrobbler) Scrobble(user *db.User, opts ScrobbleOptions) error {
	listenType := "single"
	if !opts.Submission {
		listenType = "playing_now"
	}
	scrobble := ListenBrainzScrobble{
		ListenType: listenType,
		Payload: []ListenBrainzPayload{{
			ListenedAt: opts.StampMili / 1e3,
			TrackMetadata: ListenBrainzTrackMetadata{
				AdditionalInfo: ListenBrainzAdditionalInfo{
					TrackNumber: opts.Track.TagTrackNumber,
				},
				ArtistName:  opts.Track.TagTrackArtist,
				TrackName:   opts.Track.TagTitle,
				ReleaseName: opts.Track.Album.TagTitle,
			},
		}},
	}
	payloadBuf := bytes.Buffer{}
	if err := json.NewEncoder(&payloadBuf).Encode(scrobble); err != nil {
		return err
	}
	req, _ := http.NewRequest("POST", lbBaseURL+"/1/submit-listens", &payloadBuf)
	req.Header.Add("Authorization", "Token "+user.ListenBrainzSession)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unathorized error scrobbling to listenbrainz %w",
			ErrListenBrainz)
	}
	res.Body.Close()
	return nil
}

func (lb *ListenBrainzScrobbler) Enabled(user *db.User) bool {
	return user.ListenBrainzSession != ""
}
