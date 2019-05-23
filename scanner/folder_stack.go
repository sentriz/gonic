package scanner

import "github.com/sentriz/gonic/model"

type folderStack []model.Folder

func (s *folderStack) Push(v model.Folder) {
	*s = append(*s, v)
}

func (s *folderStack) Pop() model.Folder {
	l := len(*s)
	if l == 0 {
		return model.Folder{}
	}
	r := (*s)[l-1]
	*s = (*s)[:l-1]
	return r
}

func (s *folderStack) Peek() model.Folder {
	l := len(*s)
	if l == 0 {
		return model.Folder{}
	}
	return (*s)[l-1]
}

func (s *folderStack) PeekID() int {
	l := len(*s)
	if l == 0 {
		return 0
	}
	return (*s)[l-1].ID
}
