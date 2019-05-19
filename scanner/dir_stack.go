package scanner

import (
	"github.com/sentriz/gonic/model"
)

type dirStack []*model.Folder

func (s *dirStack) Push(v *model.Folder) {
	*s = append(*s, v)
}

func (s *dirStack) Pop() *model.Folder {
	l := len(*s)
	if l == 0 {
		return nil
	}
	r := (*s)[l-1]
	*s = (*s)[:l-1]
	return r
}

func (s *dirStack) Peek() *model.Folder {
	l := len(*s)
	if l == 0 {
		return nil
	}
	return (*s)[l-1]
}

func (s *dirStack) PeekID() int {
	l := len(*s)
	if l == 0 {
		return 0
	}
	return (*s)[l-1].ID
}
