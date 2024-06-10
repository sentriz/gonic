package listenwith

import (
	set "github.com/deckarep/golang-set/v2"

	"go.senan.xyz/gonic/db"
)

type ListenWith map[string]set.Set[string]

type ListenerGraph struct {
	buddies  ListenWith
	inverted ListenWith // An inverted index of listeners to the users they are listening along with
}

func NewListenerGraph() *ListenerGraph {
	return &ListenerGraph{
		buddies:  make(ListenWith),
		inverted: make(ListenWith),
	}
}

func (g *ListenerGraph) AddUser(u *db.User) {
	if g.buddies[u.Name] == nil {
		g.buddies[u.Name] = set.NewSet[string]()
	}
}

func (g *ListenerGraph) AddListener(u, l *db.User) {
	if g.buddies[u.Name] == nil {
		g.AddUser(u)
	}

	g.buddies[u.Name].Add(l.Name)
	g.updateInverted(l, u, "add")
}

func (g *ListenerGraph) RemoveListener(u, l *db.User) {
	if g.buddies[u.Name] == nil {
		g.AddUser(u)
	}

	g.buddies[u.Name].Remove(l.Name)
	g.updateInverted(l, u, "rem")
}

func (g *ListenerGraph) GetListeners(u *db.User) set.Set[string] {
	return g.buddies[u.Name]
}

func (g *ListenerGraph) GetListenersSlice(u *db.User) []string {
	return g.buddies[u.Name].ToSlice()
}

func (g *ListenerGraph) updateInverted(l, u *db.User, op string) {
	if g.inverted[l.Name] == nil {
		g.inverted[l.Name] = set.NewSet[string]()
	}
	switch op {
	case "add":
		g.inverted[l.Name].Add(u.Name)
	case "rem":
		g.inverted[l.Name].Remove(u.Name)
	default:
		break
	}
}

func (g *ListenerGraph) GetInverted(u *db.User) set.Set[string] {
	return g.inverted[u.Name]
}

func (g *ListenerGraph) GetInvertedSlice(u *db.User) []string {
	return g.inverted[u.Name].ToSlice()
}
