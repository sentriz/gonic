package listenwith

import (
	set "github.com/deckarep/golang-set/v2"

	"go.senan.xyz/gonic/db"
)

type ListenWith map[string]set.Set[string]

var (
	Buddies  ListenWith = make(ListenWith)
	Inverted ListenWith = make(ListenWith) // An inverted index of listeners to the users they are listening along with
)

func (lw *ListenWith) AddUser(u *db.User) {
	if (*lw)[u.Name] == nil {
		(*lw)[u.Name] = set.NewSet[string]()
	}
}

func (lw *ListenWith) AddListener(u, l *db.User) {
	if (*lw)[u.Name] == nil {
		lw.AddUser(u)
	}

	(*lw)[u.Name].Add(l.Name)
	updateInverted(l, u, "add")
}

func (lw *ListenWith) RemoveListener(u, l *db.User) {
	if (*lw)[u.Name] == nil {
		lw.AddUser(u)
	}

	(*lw)[u.Name].Remove(l.Name)
	updateInverted(l, u, "rem")
}

func updateInverted(l, u *db.User, op string) {
	if Inverted[l.Name] == nil {
		Inverted[l.Name] = set.NewSet[string]()
	}
	switch op {
	case "add":
		Inverted[l.Name].Add(u.Name)
	case "rem":
		Inverted[l.Name].Remove(u.Name)
	default:
		break
	}
}
