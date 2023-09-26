package lastfm

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scrobble"
)

type Scrobbler struct {
	db *db.DB
	*Client
}

var _ scrobble.Scrobbler = (*Scrobbler)(nil)

// TODO: remove dependency on db here
func NewScrobbler(db *db.DB, client *Client) *Scrobbler {
	return &Scrobbler{
		db:     db,
		Client: client,
	}
}

func (s *Scrobbler) Scrobble(user *db.User, track *db.Track, stamp time.Time, submission bool) error {
	if user.LastFMSession == "" {
		return nil
	}
	if track.Album == nil || len(track.Album.Artists) == 0 {
		return fmt.Errorf("track has no album artists")
	}

	apiKey, err := s.db.GetSetting(db.LastFMAPIKey)
	if err != nil {
		return fmt.Errorf("get api key: %w", err)
	}
	secret, err := s.db.GetSetting(db.LastFMSecret)
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
	params.Add("albumArtist", strings.Join(track.Album.ArtistsStrings(), ", "))
	params.Add("duration", strconv.Itoa(track.Length))

	// make sure we provide a valid uuid, since some users may have an incorrect mbid in their tags
	if _, err := uuid.Parse(track.TagBrainzID); err == nil {
		params.Add("mbid", track.TagBrainzID)
	}

	params.Add("api_sig", getParamSignature(params, secret))

	_, err = s.Client.makeRequest(http.MethodPost, params)
	return err
}

func (s *Scrobbler) LoveTrack(user *db.User, track *db.Track) error {
	if user.LastFMSession == "" {
		return nil
	}

	apiKey, err := s.db.GetSetting(db.LastFMAPIKey)
	if err != nil {
		return fmt.Errorf("get api key: %w", err)
	}
	secret, err := s.db.GetSetting(db.LastFMSecret)
	if err != nil {
		return fmt.Errorf("get secret: %w", err)
	}

	params := url.Values{}
	params.Add("method", "track.love")
	params.Add("track", track.TagTitle)
	params.Add("artist", track.TagTrackArtist)
	params.Add("api_key", apiKey)
	params.Add("sk", user.LastFMSession)
	params.Add("api_sig", getParamSignature(params, secret))

	_, err = s.makeRequest(http.MethodPost, params)
	return err
}

func (s *Scrobbler) GetCurrentUser(user *db.User) (User, error) {
	if user.LastFMSession == "" {
		return User{}, nil
	}

	apiKey, err := s.db.GetSetting(db.LastFMAPIKey)
	if err != nil {
		return User{}, fmt.Errorf("get api key: %w", err)
	}
	secret, err := s.db.GetSetting(db.LastFMSecret)
	if err != nil {
		return User{}, fmt.Errorf("get secret: %w", err)
	}

	params := url.Values{}
	params.Add("method", "user.getInfo")
	params.Add("api_key", apiKey)
	params.Add("sk", user.LastFMSession)
	params.Add("api_sig", getParamSignature(params, secret))

	resp, err := s.makeRequest(http.MethodGet, params)
	if err != nil {
		return User{}, fmt.Errorf("making user GET: %w", err)
	}
	return resp.User, nil
}
