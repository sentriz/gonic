package listenbrainz

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/server/scrobble"
)

const (
	baseURL    = "https://api.listenbrainz.org"
	submitPath = "/1/submit-listens"

	listenTypeSingle     = "single"
	listenTypePlayingNow = "playing_now"
)

var (
	ErrListenBrainz = errors.New("listenbrainz error")
)

type AdditionalInfo struct {
	TrackNumber int `json:"tracknumber"`
}

type TrackMetadata struct {
	AdditionalInfo AdditionalInfo `json:"additional_info"`
	ArtistName     string         `json:"artist_name"`
	TrackName      string         `json:"track_name"`
	ReleaseName    string         `json:"release_name"`
}

type Payload struct {
	ListenedAt    int           `json:"listened_at"`
	TrackMetadata TrackMetadata `json:"track_metadata"`
}

type Scrobble struct {
	ListenType string    `json:"listen_type"`
	Payload    []Payload `json:"payload"`
}

type Scrobbler struct{}

func (s *Scrobbler) Scrobble(user *db.User, track *db.Track, stampMili int, submission bool) error {
	if user.ListenBrainzSession == "" {
		return nil
	}
	payload := Payload{
		ListenedAt: stampMili / 1e3,
		TrackMetadata: TrackMetadata{
			AdditionalInfo: AdditionalInfo{
				TrackNumber: track.TagTrackNumber,
			},
			ArtistName:  track.TagTrackArtist,
			TrackName:   track.TagTitle,
			ReleaseName: track.Album.TagTitle,
		},
	}
	scrobble := Scrobble{
		ListenType: listenTypeSingle,
		Payload:    []Payload{payload},
	}
	if !submission {
		scrobble.ListenType = listenTypePlayingNow
	}
	payloadBuf := bytes.Buffer{}
	if err := json.NewEncoder(&payloadBuf).Encode(scrobble); err != nil {
		return err
	}
	submitURL := fmt.Sprintf("%s%s", baseURL, submitPath)
	authHeader := fmt.Sprintf("Token %s", user.ListenBrainzSession)
	req, _ := http.NewRequest("POST", submitURL, &payloadBuf)
	req.Header.Add("Authorization", authHeader)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	if res.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unathorized error scrobbling to listenbrainz %w", ErrListenBrainz)
	}
	res.Body.Close()
	return nil
}

var _ scrobble.Scrobbler = (*Scrobbler)(nil)
