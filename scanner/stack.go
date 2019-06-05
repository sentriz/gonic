package scanner

import (
	"fmt"
	"strings"

	"github.com/sentriz/gonic/model"
)

type folderStack []*model.Album

func (s *folderStack) Push(v *model.Album) {
	*s = append(*s, v)
}

func (s *folderStack) Pop() *model.Album {
	l := len(*s)
	if l == 0 {
		return nil
	}
	r := (*s)[l-1]
	*s = (*s)[:l-1]
	return r
}

func (s *folderStack) Peek() *model.Album {
	l := len(*s)
	if l == 0 {
		return nil
	}
	return (*s)[l-1]
}

func (s *folderStack) String() string {
	paths := make([]string, len(*s))
	for i, folder := range *s {
		paths[i] = folder.LeftPath
	}
	return fmt.Sprintf("[%s]", strings.Join(paths, " "))
}
