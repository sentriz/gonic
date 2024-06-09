package listenwith

import (
	set "github.com/deckarep/golang-set/v2"

	"go.senan.xyz/gonic/db"
)

type ListenWith map[string]set.Set[string]

var (
	buddies  ListenWith = make(ListenWith)
	inverted ListenWith = make(ListenWith) // An inverted index of listeners to the users they are listening along with
)

func AddUser(u *db.User) {
	if buddies[u.Name] == nil {
		buddies[u.Name] = set.NewSet[string]()
	}
}

func AddListener(u, l *db.User) {
	if buddies[u.Name] == nil {
		AddUser(u)
	}

	buddies[u.Name].Add(l.Name)
	updateInverted(l, u, "add")
}

func RemoveListener(u, l *db.User) {
	if buddies[u.Name] == nil {
		AddUser(u)
	}

	buddies[u.Name].Remove(l.Name)
	updateInverted(l, u, "rem")
}

func GetListeners(u *db.User) set.Set[string] {
	return buddies[u.Name]
}

func GetListenersSlice(u *db.User) []string {
	return buddies[u.Name].ToSlice()
}

func updateInverted(l, u *db.User, op string) {
	if inverted[l.Name] == nil {
		inverted[l.Name] = set.NewSet[string]()
	}
	switch op {
	case "add":
		inverted[l.Name].Add(u.Name)
	case "rem":
		inverted[l.Name].Remove(u.Name)
	default:
		break
	}
}

func GetInverted(u *db.User) set.Set[string] {
	return inverted[u.Name]
}

func GetInvertedSlice(u *db.User) []string {
	return inverted[u.Name].ToSlice()
}
