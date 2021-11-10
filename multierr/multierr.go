package multierr

import "strings"

type Err struct {
	errs []error
}

func (me *Err) Error() string {
	var builder strings.Builder
	for _, err := range me.errs {
		builder.WriteString("\n")
		builder.WriteString(err.Error())
	}
	return builder.String()
}

func (me *Err) Errors() []error {
	return me.errs
}

func (me *Err) Len() int {
	return len(me.errs)
}

func (me *Err) Add(err error) {
	me.errs = append(me.errs, err)
}

func (me *Err) Extend(errs []error) {
	me.errs = append(me.errs, errs...)
}
