package specid

import (
	"errors"
	"testing"
)

func TestParseID(t *testing.T) {
	tcases := []struct {
		param    string
		expType  IDT
		expValue int
		expErr   error
	}{
		{param: "al-45", expType: Album, expValue: 45},
		{param: "ar-2", expType: Artist, expValue: 2},
		{param: "tr-43", expType: Track, expValue: 43},
		{param: "xx-1", expErr: ErrBadPrefix},
		{param: "al-howdy", expErr: ErrNotAnInt},
	}
	for _, tcase := range tcases {
		tcase := tcase // pin
		t.Run(tcase.param, func(t *testing.T) {
			act, err := New(tcase.param)
			if !errors.Is(err, tcase.expErr) {
				t.Fatalf("expected err %q, got %q", tcase.expErr, err)
			}
			if act.Value != tcase.expValue {
				t.Errorf("expected value %d, got %d", tcase.expValue, act.Value)
			}
			if act.Type != tcase.expType {
				t.Errorf("expected type %v, got %v", tcase.expType, act.Type)
			}
		})
	}
}
