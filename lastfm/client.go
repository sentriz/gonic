package lastfm

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/andybalholm/cascadia"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scrobble"
	"golang.org/x/net/html"
)

var (
	ErrLastFM        = errors.New("last.fm error")
	ErrNoUserSession = errors.New("no lastfm user session present")
)

type KeySecretFunc func() (apiKey, secret string, err error)

type Client struct {
	httpClient *http.Client
	keySecret  KeySecretFunc
}

func NewClient(keySecret KeySecretFunc) *Client {
	return NewClientCustom(http.DefaultClient, keySecret)
}

func NewClientCustom(httpClient *http.Client, keySecret KeySecretFunc) *Client {
	return &Client{httpClient: httpClient, keySecret: keySecret}
}

const (
	BaseURL = "https://ws.audioscrobbler.com/2.0/"
)

func (c *Client) ArtistGetInfo(artistName string) (Artist, error) {
	apiKey, _, err := c.keySecret()
	if err != nil {
		return Artist{}, fmt.Errorf("get key and secret: %w", err)
	}

	params := url.Values{}
	params.Add("method", "artist.getInfo")
	params.Add("api_key", apiKey)
	params.Add("artist", artistName)

	resp, err := c.makeRequest(http.MethodGet, params)
	if err != nil {
		return Artist{}, fmt.Errorf("make request: %w", err)
	}
	return resp.Artist, nil
}

func (c *Client) ArtistGetTopTracks(artistName string) (TopTracks, error) {
	apiKey, _, err := c.keySecret()
	if err != nil {
		return TopTracks{}, fmt.Errorf("get key and secret: %w", err)
	}

	params := url.Values{}
	params.Add("method", "artist.getTopTracks")
	params.Add("api_key", apiKey)
	params.Add("artist", artistName)

	resp, err := c.makeRequest(http.MethodGet, params)
	if err != nil {
		return TopTracks{}, fmt.Errorf("make request: %w", err)
	}
	return resp.TopTracks, nil
}

func (c *Client) TrackGetSimilarTracks(artistName, trackName string) (SimilarTracks, error) {
	apiKey, _, err := c.keySecret()
	if err != nil {
		return SimilarTracks{}, fmt.Errorf("get key and secret: %w", err)
	}

	params := url.Values{}
	params.Add("method", "track.getSimilar")
	params.Add("api_key", apiKey)
	params.Add("track", trackName)
	params.Add("artist", artistName)

	resp, err := c.makeRequest(http.MethodGet, params)
	if err != nil {
		return SimilarTracks{}, fmt.Errorf("make request: %w", err)
	}
	return resp.SimilarTracks, nil
}

func (c *Client) ArtistGetSimilar(artistName string) (SimilarArtists, error) {
	apiKey, _, err := c.keySecret()
	if err != nil {
		return SimilarArtists{}, fmt.Errorf("get key and secret: %w", err)
	}

	params := url.Values{}
	params.Add("method", "artist.getSimilar")
	params.Add("api_key", apiKey)
	params.Add("artist", artistName)

	resp, err := c.makeRequest(http.MethodGet, params)
	if err != nil {
		return SimilarArtists{}, fmt.Errorf("making similar artists GET:  %w", err)
	}
	return resp.SimilarArtists, nil
}

func (c *Client) UserGetLovedTracks(userName string) (LovedTracks, error) {
	apiKey, _, err := c.keySecret()
	if err != nil {
		return LovedTracks{}, fmt.Errorf("get key and secret: %w", err)
	}

	params := url.Values{}
	params.Add("method", "user.getLovedTracks")
	params.Add("api_key", apiKey)
	params.Add("user", userName)
	params.Add("limit", "1000") // TODO: paginate

	resp, err := c.makeRequest(http.MethodGet, params)
	if err != nil {
		return LovedTracks{}, fmt.Errorf("making user get loved tracks GET:  %w", err)
	}
	return resp.LovedTracks, nil
}

func (c *Client) GetSession(token string) (string, error) {
	apiKey, secret, err := c.keySecret()
	if err != nil {
		return "", fmt.Errorf("get key and secret: %w", err)
	}

	params := url.Values{}
	params.Add("method", "auth.getSession")
	params.Add("api_key", apiKey)
	params.Add("token", token)
	params.Add("api_sig", GetParamSignature(params, secret))

	resp, err := c.makeRequest(http.MethodGet, params)
	if err != nil {
		return "", fmt.Errorf("make request: %w", err)
	}
	return resp.Session.Key, nil
}

//nolint:gochecknoglobals
var artistOpenGraphQuery = cascadia.MustCompile(`html > head > meta[property="og:image"]`)

func (c *Client) StealArtistImage(artistURL string) (string, error) {
	resp, err := c.httpClient.Get(artistURL) //nolint:gosec
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

func (c *Client) IsUserAuthenticated(user db.User) bool {
	return user.LastFMSession != ""
}

func (c *Client) Scrobble(user db.User, track scrobble.Track, stamp time.Time, submission bool) error {
	apiKey, secret, err := c.keySecret()
	if err != nil {
		return fmt.Errorf("get key and secret: %w", err)
	}
	if !c.IsUserAuthenticated(user) {
		return ErrNoUserSession
	}

	params := url.Values{}
	if submission {
		params.Add("method", "track.Scrobble")
		params.Add("timestamp", strconv.Itoa(int(stamp.Unix()))) // last.fm wants the timestamp in seconds
	} else {
		params.Add("method", "track.updateNowPlaying")
	}

	params.Add("artist", track.Artist)
	params.Add("track", track.Track)
	params.Add("trackNumber", strconv.Itoa(int(track.TrackNumber)))
	params.Add("album", track.Album)
	params.Add("albumArtist", track.AlbumArtist)
	params.Add("duration", strconv.Itoa(int(track.Duration.Seconds())))

	if track.MusicBrainzID != "" {
		params.Add("mbid", track.MusicBrainzID)
	}

	params.Add("sk", user.LastFMSession)
	params.Add("api_key", apiKey)
	params.Add("api_sig", GetParamSignature(params, secret))

	_, err = c.makeRequest(http.MethodPost, params)
	return err
}

func (c *Client) LoveTrack(user *db.User, track *db.Track) error {
	apiKey, secret, err := c.keySecret()
	if err != nil {
		return fmt.Errorf("get key and secret: %w", err)
	}
	if !c.IsUserAuthenticated(*user) {
		return ErrNoUserSession
	}

	params := url.Values{}
	params.Add("method", "track.love")
	params.Add("track", track.TagTitle)
	params.Add("artist", track.TagTrackArtist)
	params.Add("api_key", apiKey)
	params.Add("sk", user.LastFMSession)
	params.Add("api_sig", GetParamSignature(params, secret))

	_, err = c.makeRequest(http.MethodPost, params)
	return err
}

func (c *Client) GetCurrentUser(user *db.User) (User, error) {
	apiKey, secret, err := c.keySecret()
	if err != nil {
		return User{}, fmt.Errorf("get key and secret: %w", err)
	}
	if !c.IsUserAuthenticated(*user) {
		return User{}, ErrNoUserSession
	}

	params := url.Values{}
	params.Add("method", "user.getInfo")
	params.Add("api_key", apiKey)
	params.Add("sk", user.LastFMSession)
	params.Add("api_sig", GetParamSignature(params, secret))

	resp, err := c.makeRequest(http.MethodGet, params)
	if err != nil {
		return User{}, fmt.Errorf("make request: %w", err)
	}
	return resp.User, nil
}

func (c *Client) makeRequest(method string, params url.Values) (LastFM, error) {
	req, _ := http.NewRequest(method, BaseURL, nil)
	req.URL.RawQuery = params.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return LastFM{}, fmt.Errorf("get: %w", err)
	}
	defer resp.Body.Close()

	var lastfm LastFM
	if err = xml.NewDecoder(resp.Body).Decode(&lastfm); err != nil {
		return LastFM{}, fmt.Errorf("decoding: %w", err)
	}

	if lastfm.Error.Code != 0 {
		return LastFM{}, fmt.Errorf("%v: %w", lastfm.Error.Value, ErrLastFM)
	}
	return lastfm, nil
}

func GetParamSignature(params url.Values, secret string) string {
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
