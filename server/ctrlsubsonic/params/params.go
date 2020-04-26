// package params provides methods on url.Values for parsing params for the subsonic api
//
// the format of the functions are:
//     `Get[First|Or|FirstOr][Int|ID|Bool][List]`
//
// first component (key selection):
//     ""        -> lookup the key as usual, err if not found
//     "First"   -> lookup from a list of possible keys, err if none found
//     "Or"      -> lookup the key as usual, return `or` if not found
//     "FirstOr" -> lookup from a list of possible keys, return `or` if not found
//
// second component (type selection):
//     ""     -> parse the value as a string
//     "Int"  -> parse the value as an integer
//     "ID"   -> parse the value as an artist, track, album etc id
//     "Bool" -> parse the value as a boolean
//
// last component (list parsing with stacked keys, eg. `?a=1&a=2&a=3`):
//     ""     -> return the first value, eg. `1`
//     "List" -> return all values, eg. `{1, 2, 3}`
//
// note: these bulk of these funcs were generated with vim macros, so let me know if
// you see something wrong :)

package params

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var (
	ErrKeyNotFound = errors.New("key(s) not found")
	ErrIDInvalid   = errors.New("invalid id")
	ErrIDNotAnInt  = errors.New("not an int")
	ErrIDNotABool  = errors.New("not an bool")
)

const IDSeparator = "-"

type IDType string

const (
	// type values copied from subsonic
	IDTypeArtist IDType = "ar"
	IDTypeAlbum  IDType = "al"
	IDTypeTrack  IDType = "tr"
)

type ID struct {
	Type  IDType
	Value int
}

func IDArtist(id int) string { return fmt.Sprintf("%d-%s", id, IDTypeArtist) }
func IDAlbum(id int) string  { return fmt.Sprintf("%d-%s", id, IDTypeAlbum) }
func IDTrack(id int) string  { return fmt.Sprintf("%d-%s", id, IDTypeTrack) }

// ** begin type parsing, support {[],}{string,int,ID} => 6 types

func parse(values []string, i interface{}) error {
	if len(values) == 0 {
		return ErrKeyNotFound
	}
	var err error
	switch v := i.(type) {
	case *string:
		*v, err = parseStr(values[0])
	case *int:
		*v, err = parseInt(values[0])
	case *ID:
		*v, err = parseID(values[0])
	case *bool:
		*v, err = parseBool(values[0])
	case *[]string:
		for _, val := range values {
			parsed, err := parseStr(val)
			if err != nil {
				return err
			}
			*v = append(*v, parsed)
		}
	case *[]int:
		for _, val := range values {
			parsed, err := parseInt(val)
			if err != nil {
				return err
			}
			*v = append(*v, parsed)
		}
	case *[]ID:
		for _, val := range values {
			parsed, err := parseID(val)
			if err != nil {
				return err
			}
			*v = append(*v, parsed)
		}
	case *[]bool:
		for _, val := range values {
			parsed, err := parseBool(val)
			if err != nil {
				return err
			}
			*v = append(*v, parsed)
		}
	}
	return err
}

// ** begin parse funcs

func parseStr(in string) (string, error) {
	return in, nil
}

func parseInt(in string) (int, error) {
	if v, err := strconv.Atoi(in); err == nil {
		return v, nil
	}
	return 0, ErrIDNotAnInt
}

func parseID(in string) (ID, error) {
	parts := strings.Split(in, IDSeparator)
	if len(parts) != 2 {
		return ID{}, fmt.Errorf("bad separator: %w", ErrIDInvalid)
	}
	partType := parts[0]
	partValue := parts[1]
	val, err := parseInt(partValue)
	if err != nil {
		return ID{}, fmt.Errorf("%s: %w", partValue, err)
	}
	switch partType {
	case string(IDTypeArtist):
		return ID{Type: IDTypeArtist, Value: val}, nil
	case string(IDTypeAlbum):
		return ID{Type: IDTypeAlbum, Value: val}, nil
	case string(IDTypeTrack):
		return ID{Type: IDTypeTrack, Value: val}, nil
	}
	return ID{}, ErrIDInvalid
}

func parseBool(in string) (bool, error) {
	if v, err := strconv.ParseBool(in); err == nil {
		return v, nil
	}
	return false, ErrIDNotABool
}

type Params url.Values

func New(r *http.Request) Params {
	// first load params from the url
	params := r.URL.Query()
	// also if there's any in the post body, use those too
	if err := r.ParseForm(); err == nil {
		for k, v := range r.Form {
			params[k] = v
		}
	}
	return Params(params)
}

func (p Params) get(key string) []string {
	return p[key]
}

func (p Params) getFirst(keys []string) []string {
	for _, k := range keys {
		if v, ok := p[k]; ok {
			return v
		}
	}
	return nil
}

// ** begin str {get, get first, get or, get first or}

func (p Params) Get(key string) (string, error) {
	var ret string
	return ret, parse(p.get(key), &ret)
}

func (p Params) GetFirst(keys ...string) (string, error) {
	var ret string
	return ret, parse(p.getFirst(keys), &ret)
}

func (p Params) GetOr(key string, or string) string {
	var ret string
	if err := parse(p.get(key), &ret); err == nil {
		return ret
	}
	return or
}

func (p Params) GetFirstOr(or string, keys ...string) string {
	var ret string
	if err := parse(p.getFirst(keys), &ret); err == nil {
		return ret
	}
	return or
}

// ** begin []str {get, get first, get or, get first or}

func (p Params) GetList(key string) ([]string, error) {
	var ret []string
	return ret, parse(p.get(key), &ret)
}

func (p Params) GetFirstList(keys ...string) ([]string, error) {
	var ret []string
	return ret, parse(p.getFirst(keys), &ret)
}

func (p Params) GetOrList(key string, or []string) []string {
	var ret []string
	if err := parse(p.get(key), &ret); err == nil {
		return ret
	}
	return or
}

func (p Params) GetFirstOrList(or []string, keys ...string) []string {
	var ret []string
	if err := parse(p.getFirst(keys), &ret); err == nil {
		return ret
	}
	return or
}

// ** begin int {get, get first, get or, get first or}

func (p Params) GetInt(key string) (int, error) {
	var ret int
	return ret, parse(p.get(key), &ret)
}

func (p Params) GetFirstInt(keys ...string) (int, error) {
	var ret int
	return ret, parse(p.getFirst(keys), &ret)
}

func (p Params) GetOrInt(key string, or int) int {
	var ret int
	if err := parse(p.get(key), &ret); err == nil {
		return ret
	}
	return or
}

func (p Params) GetFirstOrInt(or int, keys ...string) int {
	var ret int
	if err := parse(p.getFirst(keys), &ret); err == nil {
		return ret
	}
	return or
}

// ** begin []int {get, get first, get or, get first or}

func (p Params) GetIntList(key string) ([]int, error) {
	var ret []int
	return ret, parse(p.get(key), &ret)
}

func (p Params) GetFirstIntList(keys ...string) ([]int, error) {
	var ret []int
	return ret, parse(p.getFirst(keys), &ret)
}

func (p Params) GetOrIntList(key string, or []int) []int {
	var ret []int
	if err := parse(p.get(key), &ret); err == nil {
		return ret
	}
	return or
}

func (p Params) GetFirstOrIntList(or []int, keys ...string) []int {
	var ret []int
	if err := parse(p.getFirst(keys), &ret); err == nil {
		return ret
	}
	return or
}

// ** begin ID {get, get first, get or, get first or}

func (p Params) GetID(key string) (ID, error) {
	var ret ID
	return ret, parse(p.get(key), &ret)
}

func (p Params) GetIDDefault() (ID, error) {
	var ret ID
	return ret, parse(p.get("id"), &ret)
}

func (p Params) GetFirstID(keys ...string) (ID, error) {
	var ret ID
	return ret, parse(p.getFirst(keys), &ret)
}

func (p Params) GetOrID(key string, or ID) ID {
	var ret ID
	if err := parse(p.get(key), &ret); err == nil {
		return ret
	}
	return or
}

func (p Params) GetFirstOrID(or ID, keys ...string) ID {
	var ret ID
	if err := parse(p.getFirst(keys), &ret); err == nil {
		return ret
	}
	return or
}

// ** begin []ID {get, get first, get or, get first or}

func (p Params) GetIDList(key string) ([]ID, error) {
	var ret []ID
	return ret, parse(p.get(key), &ret)
}

func (p Params) GetFirstIDList(keys ...string) ([]ID, error) {
	var ret []ID
	return ret, parse(p.getFirst(keys), &ret)
}

func (p Params) GetOrIDList(key string, or []ID) []ID {
	var ret []ID
	if err := parse(p.get(key), &ret); err == nil {
		return ret
	}
	return or
}

func (p Params) GetFirstOrIDList(or []ID, keys ...string) []ID {
	var ret []ID
	if err := parse(p.getFirst(keys), &ret); err == nil {
		return ret
	}
	return or
}

// ** begin bool {get, get first, get or, get first or}

func (p Params) GetBool(key string) (bool, error) {
	var ret bool
	return ret, parse(p.get(key), &ret)
}

func (p Params) GetFirstBool(keys ...string) (bool, error) {
	var ret bool
	return ret, parse(p.getFirst(keys), &ret)
}

func (p Params) GetOrBool(key string, or bool) bool {
	var ret bool
	if err := parse(p.get(key), &ret); err == nil {
		return ret
	}
	return or
}

func (p Params) GetFirstOrBool(or bool, keys ...string) bool {
	var ret bool
	if err := parse(p.getFirst(keys), &ret); err == nil {
		return ret
	}
	return or
}

// ** begin []bool {get, get first, get or, get first or}

func (p Params) GetBoolList(key string) ([]bool, error) {
	var ret []bool
	return ret, parse(p.get(key), &ret)
}

func (p Params) GetFirstBoolList(keys ...string) ([]bool, error) {
	var ret []bool
	return ret, parse(p.getFirst(keys), &ret)
}

func (p Params) GetOrBoolList(key string, or []bool) []bool {
	var ret []bool
	if err := parse(p.get(key), &ret); err == nil {
		return ret
	}
	return or
}

func (p Params) GetFirstOrBoolList(or []bool, keys ...string) []bool {
	var ret []bool
	if err := parse(p.getFirst(keys), &ret); err == nil {
		return ret
	}
	return or
}
