package multierr

import "strings"

type Err []error

func (me Err) Error() string {
	var builder strings.Builder
	for _, err := range me {
		builder.WriteString("\n")
		builder.WriteString(err.Error())
	}
	return builder.String()
}

func (me Err) Len() int {
	return len(me)
}

func (me *Err) Add(err error) {
	*me = append(*me, err)
}
