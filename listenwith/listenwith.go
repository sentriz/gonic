package listenwith

import (
	"sync"

	set "github.com/deckarep/golang-set/v2"
	"go.senan.xyz/gonic/db"
)

type (
	listenSet     map[string]set.Set[string]
	ListenerGroup struct {
		mu       sync.Mutex
		buddies  listenSet
		inverted listenSet
	}
)

func NewListenerGraph() *ListenerGroup {
	return &ListenerGroup{
		buddies:  make(listenSet),
		inverted: make(listenSet),
	}
}

func (lg *ListenerGroup) AddUser(u *db.User) {
	if lg.buddies[u.Name] == nil {
		lg.buddies[u.Name] = set.NewSet[string]()
	}
}

func (lg *ListenerGroup) AddListener(u, l *db.User) {
	lg.mu.Lock()
	defer lg.mu.Unlock()
	if lg.buddies[u.Name] == nil {
		lg.AddUser(u)
	}
	lg.buddies[u.Name].Add(l.Name)
	// add to inverted index
	if lg.inverted[l.Name] == nil {
		lg.inverted[l.Name] = set.NewSet[string]()
	}
	lg.inverted[l.Name].Add(u.Name)
}

func (lg *ListenerGroup) RemoveListener(u, l *db.User) {
	if lg.buddies[u.Name] == nil {
		lg.AddUser(u)
	}

	lg.buddies[u.Name].Remove(l.Name)
	// remove from inverted index
	if lg.inverted[l.Name] == nil {
		lg.inverted[l.Name] = set.NewSet[string]()
	}
	lg.inverted[l.Name].Remove(u.Name)
}

func (lg *ListenerGroup) GetListeners(u *db.User) []string {
	if lg.buddies[u.Name] == nil {
		return []string{}
	}
	return lg.buddies[u.Name].ToSlice()
}

func (lg *ListenerGroup) GetInverted(u *db.User) []string {
	if lg.inverted[u.Name] == nil {
		return []string{}
	}
	return lg.inverted[u.Name].ToSlice()

}
