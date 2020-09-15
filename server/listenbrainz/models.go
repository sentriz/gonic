package listenbrainz

import "go.senan.xyz/gonic/server/db"

// ScrobbleOptions contains the track info, timestamp when listening started,
// and whether it's a submission or nowplaying scrobble
type ScrobbleOptions struct {
	Track          *db.Track
	UnixTimestampS int64
	Submission     bool
}
