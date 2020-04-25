package params

import (
	"errors"
	"testing"
)

func TestParseID(t *testing.T) {
	tcases := []struct {
		param    string
		expType  IDType
		expValue int
		expErr   error
	}{
		{param: "al-45", expType: IDTypeAlbum, expValue: 45},
		{param: "ar-2", expType: IDTypeArtist, expValue: 2},
		{param: "tr-43", expType: IDTypeTrack, expValue: 43},
		{param: "xx-1", expErr: ErrIDInvalid},
		{param: "al-howdy", expErr: ErrIDNotAnInt},
	}
	for _, tcase := range tcases {
		t.Run(tcase.param, func(t *testing.T) {
			act, err := parseID(tcase.param)
			if err != nil && !errors.Is(err, tcase.expErr) {
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

// TODO?
func TestGet(t *testing.T)             {}
func TestGetFirst(t *testing.T)        {}
func TestGetOr(t *testing.T)           {}
func TestGetList(t *testing.T)         {}
func TestGetFirstList(t *testing.T)    {}
func TestGetOrList(t *testing.T)       {}
func TestGetInt(t *testing.T)          {}
func TestGetFirstInt(t *testing.T)     {}
func TestGetOrInt(t *testing.T)        {}
func TestGetIntList(t *testing.T)      {}
func TestGetFirstIntList(t *testing.T) {}
func TestGetOrIntList(t *testing.T)    {}
func TestGetID(t *testing.T)           {}
func TestGetFirstID(t *testing.T)      {}
func TestGetOrID(t *testing.T)         {}
func TestGetIDList(t *testing.T)       {}
func TestGetFirstIDList(t *testing.T)  {}
func TestGetOrIDList(t *testing.T)     {}
