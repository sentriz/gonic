package listenwith

import (
	"testing"

	set "github.com/deckarep/golang-set/v2"

	"go.senan.xyz/gonic/db"
)

// TestAddUser tests that multiple concurrent calls to add the same user don't
// result in unexpected behavior
func TestAddUser(t *testing.T) {
	t.Parallel()
	var (
		u        = db.User{Name: "gonic", ID: 0}
		b        = db.User{Name: "buddy", ID: 1}
		expected = set.NewSet(b.Name)
		lg       = NewListenerGraph()
	)
	for i := 1; i <= 60; i++ {
		go func(i int) {
			if i%2 == 0 {
				lg.AddUser(&u)
			} else {
				lg.AddListener(&u, &b)
			}
		}(i)
	}
	if !lg.GetListeners(&u).Equal(expected) {
		t.Fatalf("expected concurrent calls of AddUser() to be gracefully managed\nexpected: %s\nactual: %s", expected, lg.GetListeners(&u))
	}
}

// TestRemoveListener tests that removing a listener updates the Buddies list and the Inverted list
func TestRemoveListener(t *testing.T) {
	t.Parallel()
	var (
		u        = db.User{Name: "gonic", ID: 0}
		b        = db.User{Name: "buddy", ID: 1}
		expected = set.NewSet[string]()
		inverted = set.NewSet[string]()
		lg       = NewListenerGraph()
	)
	lg.AddListener(&u, &b)
	lg.RemoveListener(&u, &b)
	if !lg.GetListeners(&u).Equal(expected) {
		t.Fatalf("expected AddListener() to add a listener\nexpected: %s\nactual: %s", expected, lg.GetListeners(&u))
	}
	if !lg.GetInverted(&b).Equal(inverted) {
		t.Fatalf("expected AddListener() to update the inverted index\nexpected: %s\nactual: %s", inverted, lg.GetInverted(&b))
	}
}

// TestAddListener tests that adding a listener updates the Buddies list and the Inverted list
func TestAddListener(t *testing.T) {
	t.Parallel()
	var (
		u        = db.User{Name: "gonic", ID: 0}
		b        = db.User{Name: "buddy", ID: 1}
		expected = set.NewSet(b.Name)
		inverted = set.NewSet(u.Name)
		lg       = NewListenerGraph()
	)
	lg.AddListener(&u, &b)
	if !lg.GetListeners(&u).Equal(expected) {
		t.Fatalf("expected AddListener() to add a listener\nexpected: %s\nactual: %s", expected, lg.GetListeners(&u))
	}
	if !lg.GetInverted(&b).Equal(inverted) {
		t.Fatalf("expected AddListener() to update the inverted index\nexpected: %s\nactual: %s", inverted, lg.GetInverted(&b))
	}
}
