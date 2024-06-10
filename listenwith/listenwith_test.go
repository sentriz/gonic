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
	// We use a WaitGroup here to ensure that some combination of AddUser() and
	// AddListener() run before GetListeners() runs. This may seem contrary to
	// testing concurrency, but what we are really interested in is whether,
	// given a concurrent initialization of a user and a request to add a
	// listener to that user, the end result is the same regardless of the order
	// these events execute in. We make no guarantees that a read of a user's
	// listeners which is concurrent with the write of a user to that list will
	// reflect that user being added. In practice the only thing this should
	// affect is whether a scrobble which happens concurrently with a user
	// registering themselves as a listener will be propagated to that user,
	// which is a relatively minor risk.
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
