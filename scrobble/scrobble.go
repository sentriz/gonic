package scrobble

import (
	"time"

	"go.senan.xyz/gonic/db"
)

type Scrobbler interface {
	Scrobble(user *db.User, track *db.Track, stamp time.Time, submission bool) error
}
