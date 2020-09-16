package listenbrainz

import (
	"errors"
	"fmt"

	"github.com/spezifisch/go-listenbrainz"
)

var (
	// ErrListenBrainz means you gave wrong parameters
	ErrListenBrainz = errors.New("listenbrainz error")
)

// Scrobble submits the track given in opts to the given ListenBrainz server if enabled
func Scrobble(listenbrainzEnabled, customURLEnabled bool, token, customURL string, opts ScrobbleOptions) error {
	if !listenbrainzEnabled {
		return nil
	}
	api := listenbrainz.GetDefaultAPI()
	api.Token = token
	if customURLEnabled && customURL != "" {
		api.URL = customURL
	}
	// required fields
	if opts.Track == nil {
		return fmt.Errorf("opts.Track is nil: %w", ErrListenBrainz)
	}
	track := listenbrainz.Track{
		Artist:         opts.Track.TagTrackArtist,
		Title:          opts.Track.TagTitle,
		AdditionalInfo: make(map[string]interface{}),
	}
	// optional
	if opts.Track.Album != nil && opts.Track.Album.TagTitle != "" {
		track.Album = opts.Track.Album.TagTitle
	}
	// optional "official" fields,
	// see https://listenbrainz.readthedocs.io/en/production/dev/json/#submission-json
	if opts.Track.TagTrackNumber > 0 {
		track.AdditionalInfo["tracknumber"] = opts.Track.TagTrackNumber
	}
	if opts.Track.TagBrainzID != "" {
		track.AdditionalInfo["track_mbid"] = opts.Track.TagBrainzID
	}
	// our custom additional fields
	if opts.Track.Artist != nil && opts.Track.Artist.Name != "" {
		track.AdditionalInfo["release_artist"] = opts.Track.Artist.Name
	}
	if opts.Track.Length > 0 {
		track.AdditionalInfo["track_length"] = opts.Track.Length
	}

	if opts.Submission {
		_, err := api.SubmitSingle(track, opts.UnixTimestampS)
		return err
	}

	_, err := api.SubmitPlayingNow(track)
	return err
}
