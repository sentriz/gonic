package listenbrainz

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"go.senan.xyz/gonic/server/db"
)

const (
	defaultURL = "https://api.listenbrainz.org"
)

var (
	ErrListenBrainz = errors.New("listenbrainz error")
)

type ScrobbleOptions struct {
	Track          *db.Track
	UnixTimestampS int
	Submission     bool
}

type trackMetadata struct {
	ArtistName     string                 `json:"artist_name"`
	TrackName      string                 `json:"track_name"`
	ReleaseName    string                 `json:"release_name, omitempty"`
	AdditionalInfo map[string]interface{} `json:"additional_info"`
}

type scrobblePayload struct {
	ListenedAt    int           `json:"listened_at, omitempty"` // missing from playing_now
	TrackMetadata trackMetadata `json:"track_metadata"`
}

type scrobbleRequest struct {
	ListenType string            `json:"listen_type"` // single, playing_now, import
	Payload    []scrobblePayload `json:"payload"`
}

func makeRequest(baseURL, token, method string, scrobble scrobbleRequest) error {
	apiURL := baseURL + "/1/submit-listens"
	jsonValue, err := json.Marshal(scrobble)
	if err != nil {
		return fmt.Errorf("json: %w", err)
	}
	postBody := bytes.NewReader(jsonValue)
	req, _ := http.NewRequest(method, apiURL, postBody)
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func Scrobble(listenbrainzEnabled, customURLEnabled bool, token, customURL string, opts ScrobbleOptions) error {
	if !listenbrainzEnabled {
		return nil
	}
	baseURL := defaultURL
	if customURLEnabled && customURL != "" {
		baseURL = customURL
	}
	// required fields
	payload := scrobblePayload{
		TrackMetadata: trackMetadata{
			ArtistName:     opts.Track.TagTrackArtist,
			TrackName:      opts.Track.TagTitle,
			AdditionalInfo: make(map[string]interface{}),
		},
	}
	// optional
	if opts.Track.Album.TagTitle != "" {
		payload.TrackMetadata.ReleaseName = opts.Track.Album.TagTitle
	}
	// optional "official" fields,
	// see https://listenbrainz.readthedocs.io/en/production/dev/json/#submission-json
	if opts.Track.TagTrackNumber > 0 {
		payload.TrackMetadata.AdditionalInfo["tracknumber"] = opts.Track.TagTrackNumber
	}
	if opts.Track.TagBrainzID != "" {
		payload.TrackMetadata.AdditionalInfo["track_mbid"] = opts.Track.TagBrainzID
	}
	// our custom additional fields
	if opts.Track.Artist.Name != "" {
		payload.TrackMetadata.AdditionalInfo["release_artist"] = opts.Track.Artist.Name
	}
	if opts.Track.Length > 0 {
		payload.TrackMetadata.AdditionalInfo["track_length"] = opts.Track.Length
	}

	scrobble := scrobbleRequest{
		Payload: make([]scrobblePayload, 1),
	}
	if opts.Submission {
		scrobble.ListenType = "single"
		payload.ListenedAt = opts.UnixTimestampS
	} else {
		// no timestamp for playing_now
		scrobble.ListenType = "playing_now"
	}
	scrobble.Payload[0] = payload

	return makeRequest(baseURL, token, "POST", scrobble)
}
