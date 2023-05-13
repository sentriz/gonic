package lastfm

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scrobble"
)

const (
	baseURL = "https://ws.audioscrobbler.com/2.0/"
)

var (
	ErrLastFM = errors.New("last.fm error")

	_ scrobble.Scrobbler = (*Scrobbler)(nil)
)

type (
	Scrobbler struct {
		client *Client
	}
)

func NewScrobbler(client *Client) *Scrobbler {
	return &Scrobbler{
		client: client,
	}
}

func (s *Scrobbler) Scrobble(user *db.User, track *db.Track, stamp time.Time, submission bool) error {
	if user.LastFMSession == "" {
		return nil
	}

	params := url.Values{}
	if submission {
		params.Add("method", "track.Scrobble")
		// last.fm wants the timestamp in seconds
		params.Add("timestamp", strconv.Itoa(int(stamp.Unix())))
	} else {
		params.Add("method", "track.updateNowPlaying")
	}
	params.Add("api_key", s.client.apiKey)
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

	params.Add("api_sig", s.client.getParamSignature(params))

	_, err := s.client.makeRequest(http.MethodPost, params)
	return err
}
