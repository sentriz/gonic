package scrobble

import (
	"go.senan.xyz/gonic/server/db"
)

type Scrobbler interface {
	Scrobble(user *db.User, track *db.Track, stampMili int, submission bool) error
}
