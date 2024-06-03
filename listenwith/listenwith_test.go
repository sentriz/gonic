package listenwith

import (
	"testing"

	set "github.com/deckarep/golang-set/v2"

	"go.senan.xyz/gonic/db"
)

// TestAddUser tests that multiple concurrent calls to add the same user don't
// result in unexpected behavior
func TestAddUser(t *testing.T) {
	t.Cleanup(resetBuddies)
	u := db.User{
		Name: "gonic",
		ID: 0,
	}

	b := db.User{
		Name: "buddy",
		ID: 1,
	}

	expected := set.NewSet(b.Name)

	for i := 1; i <= 60; i++ {
		go func(i int) {
			if i % 2 == 0 {
				Buddies.AddUser(&u)
			} else {
				Buddies.AddListener(&u, &b)
			}
		}(i)
	}

	if !Buddies[u.Name].Equal(expected) {
		t.Fatalf("expected concurrent calls of AddUser() to be gracefully managed\nexpected: %s\nactual: %s", expected, Buddies[u.Name])
	}
}

// TestRemoveListener tests that removing a listener updates the Buddies list and the Inverted list
func TestRemoveListener(t *testing.T) {
	t.Cleanup(resetBuddies)
	u := db.User{
		Name: "gonic",
		ID: 0,
	}

	b := db.User{
		Name: "buddy",
		ID: 1,
	}

	expected := set.NewSet[string]()
	inverted := set.NewSet[string]()

	Buddies.AddListener(&u, &b)
	Buddies.RemoveListener(&u, &b)
	if !Buddies[u.Name].Equal(expected) {
		t.Fatalf("expected AddListener() to add a listener\nexpected: %s\nactual: %s", expected, Buddies[u.Name])
	}
	if !Inverted[b.Name].Equal(inverted) {
		t.Fatalf("expected AddListener() to update the inverted index\nexpected: %s\nactual: %s", inverted, Inverted[b.Name])
	}
}

// TestAddListener tests that adding a listener updates the Buddies list and the Inverted list
func TestAddListener(t *testing.T) {
	t.Cleanup(resetBuddies)
	u := db.User{
		Name: "gonic",
		ID: 0,
	}

	b := db.User{
		Name: "buddy",
		ID: 1,
	}

	expected := set.NewSet(b.Name)
	inverted := set.NewSet(u.Name)

	Buddies.AddListener(&u, &b)
	if !Buddies[u.Name].Equal(expected) {
		t.Fatalf("expected AddListener() to add a listener\nexpected: %s\nactual: %s", expected, Buddies[u.Name])
	}
	if !Inverted[b.Name].Equal(inverted) {
		t.Fatalf("expected AddListener() to update the inverted index\nexpected: %s\nactual: %s", inverted, Inverted[b.Name])
	}
}

func resetBuddies() {
	Buddies = make(ListenWith)
	Inverted = make(ListenWith)
}
