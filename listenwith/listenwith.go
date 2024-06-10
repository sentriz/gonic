package listenwith

import (
	"sync"

	set "github.com/deckarep/golang-set/v2"
	"go.senan.xyz/gonic/db"
)

type (
	ListenerGroup struct {
		buddies  sync.Map
		inverted sync.Map
	}
)

func NewListenerGraph() *ListenerGroup {
	return &ListenerGroup{
		buddies:  sync.Map{},
		inverted: sync.Map{},
	}
}

func (lg *ListenerGroup) AddUser(u *db.User) {
	if _, ok := lg.buddies.Load(u.Name); !ok {
		lg.buddies.LoadOrStore(u.Name, set.NewSet[string]())
	}
}

func (lg *ListenerGroup) AddListener(u, l *db.User) {
	if _, ok := lg.buddies.Load(u.Name); !ok {
		lg.AddUser(u)
	}
	s, _ := lg.buddies.Load(u.Name)
	s.(set.Set[string]).Add(l.Name)
	// add to inverted index
	if _, ok := lg.inverted.Load(l.Name); !ok {
		lg.inverted.LoadOrStore(l.Name, set.NewSet[string]())
	}
	s, _ = lg.inverted.Load(l.Name)
	s.(set.Set[string]).Add(u.Name)
}

func (lg *ListenerGroup) RemoveListener(u, l *db.User) {
	if _, ok := lg.buddies.Load(u.Name); !ok {
		lg.AddUser(u)
	}

	s, _ := lg.buddies.Load(u.Name)
	s.(set.Set[string]).Remove(l.Name)
	// remove from inverted index
	if _, ok := lg.inverted.Load(l.Name); !ok {
		lg.inverted.LoadOrStore(l.Name, set.NewSet[string]())
	}
	s, _ = lg.inverted.Load(l.Name)
	s.(set.Set[string]).Remove(u.Name)
}

func (lg *ListenerGroup) GetListeners(u *db.User) []string {
	if _, ok := lg.buddies.Load(u.Name); !ok {
		lg.AddUser(u)
	}
	s, _ := lg.buddies.Load(u.Name)
	return s.(set.Set[string]).ToSlice()
}

func (lg *ListenerGroup) GetInverted(u *db.User) []string {
	if _, ok := lg.inverted.Load(u.Name); !ok {
		lg.inverted.LoadOrStore(u.Name, set.NewSet[string]())
	}
	s, _ := lg.inverted.Load(u.Name)
	return s.(set.Set[string]).ToSlice()
}
