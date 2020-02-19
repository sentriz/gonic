package stack

import (
	"fmt"
	"strings"

	"senan.xyz/g/gonic/db"
)

type item struct {
	value *db.Album
	next  *item
}

type Stack struct {
	top *item
	len uint
}

func (s *Stack) Push(v *db.Album) {
	s.top = &item{
		value: v,
		next:  s.top,
	}
	s.len++
}

func (s *Stack) Pop() *db.Album {
	if s.len == 0 {
		return nil
	}
	v := s.top.value
	s.top = s.top.next
	s.len--
	return v
}

func (s *Stack) Peek() *db.Album {
	if s.len == 0 {
		return nil
	}
	return s.top.value
}

func (s *Stack) PeekID() int {
	if s.len == 0 {
		return 0
	}
	return s.top.value.ID
}

func (s *Stack) String() string {
	var str strings.Builder
	str.WriteString("[")
	for i, f := uint(0), s.top; i < s.len; i++ {
		str.WriteString(fmt.Sprintf("%d, ", f.value.ID))
		f = f.next
	}
	str.WriteString("]")
	return str.String()
}
