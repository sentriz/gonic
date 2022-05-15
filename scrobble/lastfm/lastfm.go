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

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scrobble"
)

const (
	baseURL = "https://ws.audioscrobbler.com/2.0/"
)

var (
	ErrLastFM = errors.New("last.fm error")
)

type LastFM struct {
	XMLName        xml.Name       `xml:"lfm"`
	Status         string         `xml:"status,attr"`
	Session        Session        `xml:"session"`
	Error          Error          `xml:"error"`
	Artist         Artist         `xml:"artist"`
	TopTracks      TopTracks      `xml:"toptracks"`
	SimilarTracks  SimilarTracks  `xml:"similartracks"`
	SimilarArtists SimilarArtists `xml:"similarartists"`
}

type Session struct {
	Name       string `xml:"name"`
	Key        string `xml:"key"`
	Subscriber uint   `xml:"subscriber"`
}

type Error struct {
	Code  uint   `xml:"code,attr"`
	Value string `xml:",chardata"`
}

type SimilarArtist struct {
	XMLName xml.Name `xml:"artist"`
	Name    string   `xml:"name"`
	MBID    string   `xml:"mbid"`
	URL     string   `xml:"url"`
	Image   []struct {
		Text string `xml:",chardata"`
		Size string `xml:"size,attr"`
	} `xml:"image"`
	Streamable string `xml:"streamable"`
}

type Artist struct {
	XMLName xml.Name `xml:"artist"`
	Name    string   `xml:"name"`
	MBID    string   `xml:"mbid"`
	URL     string   `xml:"url"`
	Image   []struct {
		Text string `xml:",chardata"`
		Size string `xml:"size,attr"`
	} `xml:"image"`
	Streamable string `xml:"streamable"`
	Stats      struct {
		Listeners string `xml:"listeners"`
		Plays     string `xml:"plays"`
	} `xml:"stats"`
	Similar struct {
		Artists []Artist `xml:"artist"`
	} `xml:"similar"`
	Tags struct {
		Tag []ArtistTag `xml:"tag"`
	} `xml:"tags"`
	Bio ArtistBio `xml:"bio"`
}

type ArtistTag struct {
	Name string `xml:"name"`
	URL  string `xml:"url"`
}

type ArtistBio struct {
	Published string `xml:"published"`
	Summary   string `xml:"summary"`
	Content   string `xml:"content"`
}

type TopTracks struct {
	XMLName xml.Name `xml:"toptracks"`
	Artist  string   `xml:"artist,attr"`
	Tracks  []Track  `xml:"track"`
}

type SimilarTracks struct {
	XMLName xml.Name `xml:"similartracks"`
	Artist  string   `xml:"artist,attr"`
	Track   string   `xml:"track,attr"`
	Tracks  []Track  `xml:"track"`
}

type SimilarArtists struct {
	XMLName xml.Name `xml:"similarartists"`
	Artist  string   `xml:"artist,attr"`
	Artists []Artist `xml:"artist"`
}

type Track struct {
	Rank      int     `xml:"rank,attr"`
	Tracks    []Track `xml:"track"`
	Name      string  `xml:"name"`
	MBID      string  `xml:"mbid"`
	PlayCount int     `xml:"playcount"`
	Listeners int     `xml:"listeners"`
	URL       string  `xml:"url"`
	Image     []struct {
		Text string `xml:",chardata"`
		Size string `xml:"size,attr"`
	} `xml:"image"`
}

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

	// fetch user to get lastfm session
	if user.LastFMSession == "" {
		return fmt.Errorf("you don't have a last.fm session: %w", ErrLastFM)
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
	params.Add("mbid", track.TagBrainzID)
	params.Add("albumArtist", track.Artist.Name)
	params.Add("api_sig", getParamSignature(params, secret))
	_, err = makeRequest("POST", params)
	return err
}

var _ scrobble.Scrobbler = (*Scrobbler)(nil)
