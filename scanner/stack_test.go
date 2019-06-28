package scanner

import (
	"testing"

	"github.com/sentriz/gonic/model"
)

func testAlbum(id int) *model.Album {
	return &model.Album{IDBase: model.IDBase{id}}
}

func TestFolderStack(t *testing.T) {
	expected := "[6, 5, 4, root]"
	//
	sta := &folderStack{}
	sta.push(testAlbum(3))
	sta.push(testAlbum(4))
	sta.push(testAlbum(5))
	sta.push(testAlbum(6))
	actual := sta.string()
	if expected != actual {
		t.Errorf("first stack: expected string %q, got %q",
			expected, actual)
	}
	//
	sta = &folderStack{}
	sta.push(testAlbum(3))
	sta.push(testAlbum(4))
	sta.peek()
	sta.push(testAlbum(5))
	sta.push(testAlbum(6))
	sta.push(testAlbum(7))
	sta.pop()
	actual = sta.string()
	if expected != actual {
		t.Errorf("second stack: expected string %q, got %q",
			expected, actual)
	}
}
