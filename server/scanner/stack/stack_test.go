package stack

import (
	"testing"

	"go.senan.xyz/gonic/server/db"
)

func TestFolderStack(t *testing.T) {
	sta := &Stack{}
	sta.Push(&db.Album{ID: 3})
	sta.Push(&db.Album{ID: 4})
	sta.Push(&db.Album{ID: 5})
	sta.Push(&db.Album{ID: 6})
	expected := "[6, 5, 4, 3, ]"
	actual := sta.String()
	if expected != actual {
		t.Errorf("first stack: expected string "+
			"%q, got %q", expected, actual)
	}
	//
	sta = &Stack{}
	sta.Push(&db.Album{ID: 27})
	sta.Push(&db.Album{ID: 4})
	sta.Peek()
	sta.Push(&db.Album{ID: 5})
	sta.Push(&db.Album{ID: 6})
	sta.Push(&db.Album{ID: 7})
	sta.Pop()
	expected = "[6, 5, 4, 27, ]"
	actual = sta.String()
	if expected != actual {
		t.Errorf("second stack: expected string "+
			"%q, got %q", expected, actual)
	}
}
