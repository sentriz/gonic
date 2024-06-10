package listenwith

import (
	"slices"
	"sync"
	"testing"

	"go.senan.xyz/gonic/db"
)

// TestAddUser tests that multiple concurrent calls to add the same user don't
// result in unexpected behavior
func TestAddUser(t *testing.T) {
	t.Parallel()
	var (
		u        = &db.User{Name: "gonic", ID: 0}
		b        = &db.User{Name: "buddy", ID: 1}
		expected = []string{b.Name}
		lg       = NewListenerGraph()
		wg       sync.WaitGroup
		n        = 60
	)
	wg.Add(n)
	for i := 1; i <= n; i++ {
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				lg.AddUser(u)
			} else {
				lg.AddListener(u, b)
			}
		}(i)
	}
	wg.Wait()
	if ss := lg.GetListeners(u); ss != nil && !slices.Equal(ss, expected) {
		t.Fatalf("expected concurrent calls of AddUser() to be gracefully managed\nexpected: %s\nactual: %s", expected, lg.GetListeners(u))
	}
}

// TestAddListener tests that adding a listener updates the Buddies list and the Inverted list
func TestAddListener(t *testing.T) {
	t.Parallel()
	var (
		u        = db.User{Name: "gonic", ID: 0}
		b        = db.User{Name: "buddy", ID: 1}
		expected = []string{b.Name}
		inverted = []string{u.Name}
		lg       = NewListenerGraph()
	)
	lg.AddListener(&u, &b)
	if !slices.Equal(lg.GetListeners(&u), expected) {
		t.Fatalf("expected AddListener() to add a listener\nexpected: %s\nactual: %s", expected, lg.GetListeners(&u))
	}
	if !slices.Equal(lg.GetInverted(&b), inverted) {
		t.Fatalf("expected AddListener() to update the inverted index\nexpected: %s\nactual: %s", inverted, lg.GetInverted(&b))
	}
}
