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
	t.Cleanup(resetBuddies)
	u := db.User{
		Name: "gonic",
		ID:   0,
	}

	b := db.User{
		Name: "buddy",
		ID:   1,
	}

	expected := set.NewSet(b.Name)

	for i := 1; i <= 60; i++ {
		go func(i int) {
			if i%2 == 0 {
				AddUser(&u)
			} else {
				AddListener(&u, &b)
			}
		}(i)
	}

	if !GetListeners(&u).Equal(expected) {
		t.Fatalf("expected concurrent calls of AddUser() to be gracefully managed\nexpected: %s\nactual: %s", expected, GetListeners(&u))
	}
}

// TestRemoveListener tests that removing a listener updates the Buddies list and the Inverted list
func TestRemoveListener(t *testing.T) {
	t.Parallel()
	t.Cleanup(resetBuddies)
	u := db.User{
		Name: "gonic",
		ID:   0,
	}

	b := db.User{
		Name: "buddy",
		ID:   1,
	}

	expected := set.NewSet[string]()
	expectedInverted := set.NewSet[string]()

	AddListener(&u, &b)
	RemoveListener(&u, &b)
	if !GetListeners(&u).Equal(expected) {
		t.Fatalf("expected AddListener() to add a listener\nexpected: %s\nactual: %s", expected, GetListeners(&u))
	}
	if !GetInverted(&b).Equal(expectedInverted) {
		t.Fatalf("expected AddListener() to update the inverted index\nexpected: %s\nactual: %s", expectedInverted, GetInverted(&b))
	}
}

// TestAddListener tests that adding a listener updates the Buddies list and the Inverted list
func TestAddListener(t *testing.T) {
	t.Parallel()
	t.Cleanup(resetBuddies)
	u := db.User{
		Name: "gonic",
		ID:   0,
	}

	b := db.User{
		Name: "buddy",
		ID:   1,
	}

	expected := set.NewSet(b.Name)
	inverted := set.NewSet(u.Name)

	AddListener(&u, &b)
	if !GetListeners(&u).Equal(expected) {
		t.Fatalf("expected AddListener() to add a listener\nexpected: %s\nactual: %s", expected, GetListeners(&u))
	}
	if !GetInverted(&b).Equal(inverted) {
		t.Fatalf("expected AddListener() to update the inverted index\nexpected: %s\nactual: %s", inverted, GetInverted(&b))
	}
}

func resetBuddies() {
	buddies = make(ListenWith)
	inverted = make(ListenWith)
}
