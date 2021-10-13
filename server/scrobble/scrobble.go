package scrobble

import (
	"time"

	"go.senan.xyz/gonic/server/db"
)

type Scrobbler interface {
	Scrobble(user *db.User, track *db.Track, stamp time.Time, submission bool) error
}
