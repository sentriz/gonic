package params

// {get, get first, get or} * {str, int, id} * {list, not list} = 18

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

// ** begin str {get, get first, get or}

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

// ** begin []str {get, get first, get or}

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

// ** begin int {get, get first, get or}

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

// ** begin []int {get, get first, get or}

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

// ** begin ID {get, get first, get or}

func (p Params) GetID(key string) (ID, error) {
	var ret ID
	return ret, parse(p.get(key), &ret)
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

// ** begin []ID {get, get first, get or}

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
