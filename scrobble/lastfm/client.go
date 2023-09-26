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

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

const (
	baseURL = "https://ws.audioscrobbler.com/2.0/"
)

var (
	ErrLastFM = errors.New("last.fm error")

	//nolint:gochecknoglobals
	artistOpenGraphQuery = cascadia.MustCompile(`html > head > meta[property="og:image"]`)
)

type Client struct {
	httpClient *http.Client
}

func NewClientCustom(httpClient *http.Client) *Client {
	return &Client{httpClient: httpClient}
}

func NewClient() *Client {
	return NewClientCustom(http.DefaultClient)
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

func (c *Client) makeRequest(method string, params url.Values) (LastFM, error) {
	req, _ := http.NewRequest(method, baseURL, nil)
	req.URL.RawQuery = params.Encode()
	resp, err := c.httpClient.Do(req)
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

func (c *Client) ArtistGetInfo(apiKey string, artistName string) (Artist, error) {
	params := url.Values{}
	params.Add("method", "artist.getInfo")
	params.Add("api_key", apiKey)
	params.Add("artist", artistName)
	resp, err := c.makeRequest(http.MethodGet, params)
	if err != nil {
		return Artist{}, fmt.Errorf("making artist GET: %w", err)
	}
	return resp.Artist, nil
}

func (c *Client) ArtistGetTopTracks(apiKey, artistName string) (TopTracks, error) {
	params := url.Values{}
	params.Add("method", "artist.getTopTracks")
	params.Add("api_key", apiKey)
	params.Add("artist", artistName)
	resp, err := c.makeRequest(http.MethodGet, params)
	if err != nil {
		return TopTracks{}, fmt.Errorf("making track GET: %w", err)
	}
	return resp.TopTracks, nil
}

func (c *Client) TrackGetSimilarTracks(apiKey string, artistName, trackName string) (SimilarTracks, error) {
	params := url.Values{}
	params.Add("method", "track.getSimilar")
	params.Add("api_key", apiKey)
	params.Add("track", trackName)
	params.Add("artist", artistName)
	resp, err := c.makeRequest(http.MethodGet, params)
	if err != nil {
		return SimilarTracks{}, fmt.Errorf("making track GET: %w", err)
	}
	return resp.SimilarTracks, nil
}

func (c *Client) ArtistGetSimilar(apiKey string, artistName string) (SimilarArtists, error) {
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

func (c *Client) UserGetLovedTracks(apiKey string, userName string) (LovedTracks, error) {
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

func (c *Client) GetSession(apiKey, secret, token string) (string, error) {
	params := url.Values{}
	params.Add("method", "auth.getSession")
	params.Add("api_key", apiKey)
	params.Add("token", token)
	params.Add("api_sig", getParamSignature(params, secret))
	resp, err := c.makeRequest(http.MethodGet, params)
	if err != nil {
		return "", fmt.Errorf("making session GET: %w", err)
	}
	return resp.Session.Key, nil
}

func (c *Client) StealArtistImage(artistURL string) (string, error) {
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
