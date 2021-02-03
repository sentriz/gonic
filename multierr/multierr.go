package multierr

import (
	"strings"
)

type Err []error

func (me Err) Error() string {
	var s strings.Builder
	for _, err := range me {
		s.WriteString("\n")
		s.WriteString(err.Error())
	}
	return s.String()
}

func (me Err) Len() int {
	return len(me)
}

func (me *Err) Add(err error) {
	*me = append(*me, err)
}
