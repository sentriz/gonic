package stack

import (
	"testing"

	"github.com/sentriz/gonic/model"
)

func egAlbum(id int) *model.Album {
	return &model.Album{IDBase: model.IDBase{id}}
}

func TestFolderStack(t *testing.T) {
	sta := &Stack{}
	sta.Push(egAlbum(3))
	sta.Push(egAlbum(4))
	sta.Push(egAlbum(5))
	sta.Push(egAlbum(6))
	expected := "[6, 5, 4, 3, ]"
	actual := sta.String()
	if expected != actual {
		t.Errorf("first stack: expected string %q, got %q",
			expected, actual)
	}
	//
	sta = &Stack{}
	sta.Push(egAlbum(27))
	sta.Push(egAlbum(4))
	sta.Peek()
	sta.Push(egAlbum(5))
	sta.Push(egAlbum(6))
	sta.Push(egAlbum(7))
	sta.Pop()
	expected = "[6, 5, 4, 27, ]"
	actual = sta.String()
	if expected != actual {
		t.Errorf("second stack: expected string %q, got %q",
			expected, actual)
	}
}
