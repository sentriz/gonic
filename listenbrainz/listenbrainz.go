package listenbrainz

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"time"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scrobble"
)

const (
	BaseURL = "https://api.listenbrainz.org"

	submitPath           = "/1/submit-listens"
	listenTypeSingle     = "single"
	listenTypePlayingNow = "playing_now"
)

var ErrListenBrainz = errors.New("listenbrainz error")

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return NewClientCustom(http.DefaultClient)
}

func NewClientCustom(httpClient *http.Client) *Client {
	return &Client{httpClient: httpClient}
}

func (c *Client) IsUserAuthenticated(user db.User) bool {
	return user.ListenBrainzURL != "" && user.ListenBrainzToken != ""
}

func (c *Client) Scrobble(user db.User, track scrobble.Track, stamp time.Time, submission bool) error {
	payload := &Payload{
		TrackMetadata: &TrackMetadata{
			AdditionalInfo: &AdditionalInfo{
				TrackNumber:   int(track.TrackNumber),
				RecordingMBID: track.MusicBrainzID,
				Duration:      int(track.Duration.Seconds()),
			},
			ArtistName:  track.Artist,
			TrackName:   track.Track,
			ReleaseName: track.Album,
		},
	}
	scrobble := Scrobble{
		Payload: []*Payload{payload},
	}

	if submission && len(scrobble.Payload) > 0 {
		scrobble.ListenType = listenTypeSingle
		scrobble.Payload[0].ListenedAt = int(stamp.Unix())
	} else {
		scrobble.ListenType = listenTypePlayingNow
	}

	var payloadBuf bytes.Buffer
	if err := json.NewEncoder(&payloadBuf).Encode(scrobble); err != nil {
		return err
	}

	submitURL := fmt.Sprintf("%s%s", user.ListenBrainzURL, submitPath)
	authHeader := fmt.Sprintf("Token %s", user.ListenBrainzToken)

	req, _ := http.NewRequest(http.MethodPost, submitURL, &payloadBuf)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("unauthorized: %w", ErrListenBrainz)
	case resp.StatusCode >= 400:
		respBytes, _ := httputil.DumpResponse(resp, true)
		log.Printf("received bad listenbrainz response:\n%s", string(respBytes))
		return fmt.Errorf(">= 400: %d: %w", resp.StatusCode, ErrListenBrainz)
	}
	return nil
}
