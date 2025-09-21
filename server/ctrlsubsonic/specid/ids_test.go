package specid

import (
	"errors"
	"testing"
)

func TestParseID(t *testing.T) {
	t.Parallel()

	tcases := []struct {
		param    string
		expType  IDT
		expValue int
		expErr   error
	}{
		{param: "al-45", expType: Album, expValue: 45},
		{param: "ar-2", expType: Artist, expValue: 2},
		{param: "tr-43", expType: Track, expValue: 43},
		{param: "al-3", expType: Album, expValue: 3},
		{param: "xx-1", expErr: ErrBadPrefix},
		{param: "1", expErr: ErrBadSeparator},
		{param: "al-howdy", expErr: ErrNotAnInt},
	}

	for _, tcase := range tcases {
		t.Run(tcase.param, func(t *testing.T) {
			t.Parallel()

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
