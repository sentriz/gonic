package musicbrainz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const BaseURL = "https://musicbrainz.org/ws/2/"

const (
	LinkTypeIDIsPerson     = "dd9886f2-1dfe-4270-97db-283f6839a666"
	LinkTypeIDMemberOfBand = "5be4c609-9afa-4ea0-910b-12ffb71e3821"
)

const (
	DirectionForward  = "forward"
	DirectionBackward = "backward"
)

var ErrNotFound = errors.New("not found")

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
	Limiter    *rate.Limiter
}

func NewClient(userAgent string) *Client {
	return &Client{
		BaseURL:    BaseURL,
		HTTPClient: http.DefaultClient,
		UserAgent:  userAgent,
		Limiter:    rate.NewLimiter(rate.Every(time.Second), 1),
	}
}

func (c *Client) GetArtist(ctx context.Context, mbid string, inc ...string) (*Artist, error) {
	q := url.Values{}
	q.Set("fmt", "json")
	if len(inc) > 0 {
		q.Set("inc", strings.Join(inc, "+"))
	}

	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	u = u.JoinPath("artist", mbid)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("User-Agent", c.UserAgent)

	var a Artist
	if err := c.do(req, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func (c *Client) do(req *http.Request, dest any) error {
	if err := c.Limiter.Wait(req.Context()); err != nil {
		return fmt.Errorf("rate limit: %w", err)
	}

	resp, err := c.HTTPClient.Do(req) //nolint:gosec // base url is admin-configured
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode/100 != 2 {
		return StatusError(resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

type Artist struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Type           string     `json:"type"`
	SortName       string     `json:"sort-name"`
	Disambiguation string     `json:"disambiguation"`
	Relations      []Relation `json:"relations"`
}

type Relation struct {
	Type       string `json:"type"`
	TypeID     string `json:"type-id"`
	Direction  string `json:"direction"`
	TargetType string `json:"target-type"`
	Artist     Artist `json:"artist"`
}

type StatusError int

func (e StatusError) Error() string {
	return fmt.Sprintf("status %d", e)
}
