package lastfm

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/andybalholm/cascadia"
	"github.com/google/uuid"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scrobble"
	"golang.org/x/net/html"
)

const (
	baseURL = "https://ws.audioscrobbler.com/2.0/"
)

var (
	ErrLastFM = errors.New("last.fm error")
)

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
	req, _ := http.NewRequest(method, baseURL, nil)
	req.URL.RawQuery = params.Encode()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return LastFM{}, fmt.Errorf("get: %w", err)
	}
	defer resp.Body.Close()
	decoder := xml.NewDecoder(resp.Body)
	lastfm := LastFM{}
	if err = decoder.Decode(&lastfm); err != nil {
		respBytes, _ := httputil.DumpResponse(resp, true)
		log.Printf("received bad lastfm response:\n%s", string(respBytes))
		return LastFM{}, fmt.Errorf("decoding: %w", err)
	}
	if lastfm.Error.Code != 0 {
		respBytes, _ := httputil.DumpResponse(resp, true)
		log.Printf("received bad lastfm response:\n%s", string(respBytes))
		return LastFM{}, fmt.Errorf("%v: %w", lastfm.Error.Value, ErrLastFM)
	}
	return lastfm, nil
}

func ArtistGetInfo(apiKey string, artistName string) (Artist, error) {
	params := url.Values{}
	params.Add("method", "artist.getInfo")
	params.Add("api_key", apiKey)
	params.Add("artist", artistName)
	resp, err := makeRequest("GET", params)
	if err != nil {
		return Artist{}, fmt.Errorf("making artist GET: %w", err)
	}
	return resp.Artist, nil
}

func ArtistGetTopTracks(apiKey, artistName string) (TopTracks, error) {
	params := url.Values{}
	params.Add("method", "artist.getTopTracks")
	params.Add("api_key", apiKey)
	params.Add("artist", artistName)
	resp, err := makeRequest("GET", params)
	if err != nil {
		return TopTracks{}, fmt.Errorf("making track GET: %w", err)
	}
	return resp.TopTracks, nil
}

func TrackGetSimilarTracks(apiKey string, artistName, trackName string) (SimilarTracks, error) {
	params := url.Values{}
	params.Add("method", "track.getSimilar")
	params.Add("api_key", apiKey)
	params.Add("track", trackName)
	params.Add("artist", artistName)
	resp, err := makeRequest("GET", params)
	if err != nil {
		return SimilarTracks{}, fmt.Errorf("making track GET: %w", err)
	}
	return resp.SimilarTracks, nil
}

func ArtistGetSimilar(apiKey string, artistName string) (SimilarArtists, error) {
	params := url.Values{}
	params.Add("method", "artist.getSimilar")
	params.Add("api_key", apiKey)
	params.Add("artist", artistName)
	resp, err := makeRequest("GET", params)
	if err != nil {
		return SimilarArtists{}, fmt.Errorf("making similar artists GET:  %w", err)
	}
	return resp.SimilarArtists, nil
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

type Scrobbler struct {
	DB *db.DB
}

func (s *Scrobbler) Scrobble(user *db.User, track *db.Track, stamp time.Time, submission bool) error {
	if user.LastFMSession == "" {
		return nil
	}
	apiKey, err := s.DB.GetSetting("lastfm_api_key")
	if err != nil {
		return fmt.Errorf("get api key: %w", err)
	}
	secret, err := s.DB.GetSetting("lastfm_secret")
	if err != nil {
		return fmt.Errorf("get secret: %w", err)
	}

	params := url.Values{}
	if submission {
		params.Add("method", "track.Scrobble")
		// last.fm wants the timestamp in seconds
		params.Add("timestamp", strconv.Itoa(int(stamp.Unix())))
	} else {
		params.Add("method", "track.updateNowPlaying")
	}
	params.Add("api_key", apiKey)
	params.Add("sk", user.LastFMSession)
	params.Add("artist", track.TagTrackArtist)
	params.Add("track", track.TagTitle)
	params.Add("trackNumber", strconv.Itoa(track.TagTrackNumber))
	params.Add("album", track.Album.TagTitle)
	params.Add("albumArtist", track.Artist.Name)
	params.Add("duration", strconv.Itoa(track.Length))

	// make sure we provide a valid uuid, since some users may have an incorrect mbid in their tags
	if _, err := uuid.Parse(track.TagBrainzID); err == nil {
		params.Add("mbid", track.TagBrainzID)
	}

	params.Add("api_sig", getParamSignature(params, secret))

	_, err = makeRequest("POST", params)
	return err
}

var _ scrobble.Scrobbler = (*Scrobbler)(nil)

//nolint:gochecknoglobals
var artistOpenGraphQuery = cascadia.MustCompile(`html > head > meta[property="og:image"]`)

func StealArtistImage(artistURL string) (string, error) {
	resp, err := http.Get(artistURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("get artist url: %w", err)
	}
	defer resp.Body.Close()

	node, err := html.Parse(resp.Body)
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	n := cascadia.Query(node, artistOpenGraphQuery)
	if n == nil {
		return "", nil
	}

	var imageURL string
	for _, attr := range n.Attr {
		if attr.Key == "content" {
			imageURL = attr.Val
			break
		}
	}

	return imageURL, nil
}
