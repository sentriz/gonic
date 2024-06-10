package listenwith

import (
	"go.senan.xyz/gonic/db"
)

// Return a list of usernames with the current user filtered out
func ListeningCandidates(allUsers []*db.User, me string) []string {
	var r []string

	for _, u := range allUsers {
		if u.Name != me {
			r = append(r, u.Name)
		}
	}

	return r
}
