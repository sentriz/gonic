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

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

const (
	baseURL = "https://ws.audioscrobbler.com/2.0/"
)

var (
	//nolint:gochecknoglobals
	artistOpenGraphQuery = cascadia.MustCompile(`html > head > meta[property="og:image"]`)

	ErrLastFM = errors.New("last.fm error")
)

type Client struct {
	apiKey, secret string
	httpClient     *http.Client
}

func NewClient(apiKey, secret string) *Client {
	return &Client{
		apiKey:     apiKey,
		secret:     secret,
		httpClient: http.DefaultClient,
	}
}

func (c *Client) UpdateAPIKey(apiKey, secret string) {
	c.apiKey = apiKey
	c.secret = secret
}

func (c *Client) getParamSignature(params url.Values) string {
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
	toHash += c.secret
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

func (c *Client) ArtistGetInfo(artistName string) (Artist, error) {
	params := url.Values{}
	params.Add("method", "artist.getInfo")
	params.Add("api_key", c.apiKey)
	params.Add("artist", artistName)
	resp, err := c.makeRequest(http.MethodGet, params)
	if err != nil {
		return Artist{}, fmt.Errorf("making artist info GET: %w", err)
	}
	return resp.Artist, nil
}

func (c *Client) ArtistGetTopTracks(artistName string) (TopTracks, error) {
	params := url.Values{}
	params.Add("method", "artist.getTopTracks")
	params.Add("api_key", c.apiKey)
	params.Add("artist", artistName)
	resp, err := c.makeRequest("GET", params)
	if err != nil {
		return TopTracks{}, fmt.Errorf("making top tracks GET: %w", err)
	}
	return resp.TopTracks, nil
}

func (c *Client) TrackGetSimilarTracks(artistName, trackName string) (SimilarTracks, error) {
	params := url.Values{}
	params.Add("method", "track.getSimilar")
	params.Add("api_key", c.apiKey)
	params.Add("track", trackName)
	params.Add("artist", artistName)
	resp, err := c.makeRequest("GET", params)
	if err != nil {
		return SimilarTracks{}, fmt.Errorf("making similar tracks GET: %w", err)
	}
	return resp.SimilarTracks, nil
}

func (c *Client) ArtistGetSimilar(artistName string) (SimilarArtists, error) {
	params := url.Values{}
	params.Add("method", "artist.getSimilar")
	params.Add("api_key", c.apiKey)
	params.Add("artist", artistName)
	resp, err := c.makeRequest("GET", params)
	if err != nil {
		return SimilarArtists{}, fmt.Errorf("making similar artists GET:  %w", err)
	}
	return resp.SimilarArtists, nil
}

func (c *Client) GetSession(token string) (string, error) {
	params := url.Values{}
	params.Add("method", "auth.getSession")
	params.Add("api_key", c.apiKey)
	params.Add("token", token)
	params.Add("api_sig", c.getParamSignature(params))
	resp, err := c.makeRequest("GET", params)
	if err != nil {
		return "", fmt.Errorf("making session GET: %w", err)
	}
	return resp.Session.Key, nil
}

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
