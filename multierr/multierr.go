package multierr

import "strings"

type Err []error

func (me Err) Error() string {
	var strs []string
	for _, err := range me {
		strs = append(strs, err.Error())
	}
	return strings.Join(strs, "\n")
}

func (me Err) Len() int {
	return len(me)
}

func (me *Err) Add(err error) {
	*me = append(*me, err)
}
