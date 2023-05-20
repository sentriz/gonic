package lastfm

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scrobble"
)

type Scrobbler struct {
	db     *db.DB
	client *Client
}

var _ scrobble.Scrobbler = (*Scrobbler)(nil)

func NewScrobbler(db *db.DB, client *Client) *Scrobbler {
	return &Scrobbler{
		db:     db,
		client: client,
	}
}

func (s *Scrobbler) Scrobble(user *db.User, track *db.Track, stamp time.Time, submission bool) error {
	if user.LastFMSession == "" {
		return nil
	}
	apiKey, err := s.db.GetSetting("lastfm_api_key")
	if err != nil {
		return fmt.Errorf("get api key: %w", err)
	}
	secret, err := s.db.GetSetting("lastfm_secret")
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

	_, err = s.client.makeRequest("POST", params)
	return err
}
