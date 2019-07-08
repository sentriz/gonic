package stack

import (
	"testing"

	"senan.xyz/g/gonic/model"
)

func TestFolderStack(t *testing.T) {
	sta := &Stack{}
	sta.Push(&model.Album{ID: 3})
	sta.Push(&model.Album{ID: 4})
	sta.Push(&model.Album{ID: 5})
	sta.Push(&model.Album{ID: 6})
	expected := "[6, 5, 4, 3, ]"
	actual := sta.String()
	if expected != actual {
		t.Errorf("first stack: expected string "+
			"%q, got %q", expected, actual)
	}
	//
	sta = &Stack{}
	sta.Push(&model.Album{ID: 27})
	sta.Push(&model.Album{ID: 4})
	sta.Peek()
	sta.Push(&model.Album{ID: 5})
	sta.Push(&model.Album{ID: 6})
	sta.Push(&model.Album{ID: 7})
	sta.Pop()
	expected = "[6, 5, 4, 27, ]"
	actual = sta.String()
	if expected != actual {
		t.Errorf("second stack: expected string "+
			"%q, got %q", expected, actual)
	}
}
