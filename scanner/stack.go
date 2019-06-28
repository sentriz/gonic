package scanner

import (
	"fmt"
	"strings"

	"github.com/sentriz/gonic/model"
)

type folderItem struct {
	value *model.Album
	next  *folderItem
}

type folderStack struct {
	top *folderItem
	len uint
}

func (s *folderStack) push(v *model.Album) {
	s.top = &folderItem{
		value: v,
		next:  s.top,
	}
	s.len++
}

func (s *folderStack) pop() *model.Album {
	if s.len == 0 {
		return nil
	}
	v := s.top.value
	s.top = s.top.next
	s.len--
	return v
}

func (s *folderStack) peek() *model.Album {
	if s.len == 0 {
		return nil
	}
	return s.top.value
}

func (s *folderStack) peekID() int {
	if s.len == 0 {
		return 0
	}
	return s.top.value.ID
}

func (s *folderStack) string() string {
	var str strings.Builder
	str.WriteString("[")
	for i := s.top; i.next != nil; i = i.next {
		str.WriteString(fmt.Sprintf("%d, ", i.value.ID))
	}
	str.WriteString("root]")
	return str.String()
}
