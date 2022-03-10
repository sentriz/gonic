// package params provides methods on url.Values for parsing params for the subsonic api

// the format of the functions are:
//     `Get[First|Or|FirstOr][Int|ID|Bool|TimeMs][List]`

// first component (key selection):
//     ""        -> lookup the key as usual, err if not found
//     "First"   -> lookup from a list of possible keys, err if none found
//     "Or"      -> lookup the key as usual, return `or` if not found
//     "FirstOr" -> lookup from a list of possible keys, return `or` if not found

// second component (type selection):
//     ""      -> parse the value as a string
//     "Int"   -> parse the value as an integer
//     "Float" -> parse the value as a float
//     "ID"    -> parse the value as an artist, track, album etc id
//     "Bool"  -> parse the value as a boolean

// last component (list parsing with stacked keys, eg. `?a=1&a=2&a=3`):
//     ""     -> return the first value, eg. `1`
//     "List" -> return all values, eg. `{1, 2, 3}`

// note: these bulk of these funcs were generated with vim macros, so let me know if
// you see something wrong :)

package params

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

var (
	ErrNoValues = errors.New("no values provided")
)

// some thin wrappers
// may be needed when cleaning up parse() below
func parseStr(in string) (string, error)    { return in, nil }
func parseInt(in string) (int, error)       { return strconv.Atoi(in) }
func parseFloat(in string) (float64, error) { return strconv.ParseFloat(in, 64) }
func parseID(in string) (specid.ID, error)  { return specid.New(in) }
func parseBool(in string) (bool, error)     { return strconv.ParseBool(in) }

func parseTime(in string) (time.Time, error) {
	ms, err := strconv.Atoi(in)
	if err != nil {
		return time.Time{}, err
	}
	ns := int64(ms) * 1e6
	return time.Unix(0, ns), nil
}

func parse(values []string, i interface{}) error {
	if len(values) == 0 {
		return ErrNoValues
	}
	var err error
	switch v := i.(type) {

	// *T
	case *string:
		*v, err = parseStr(values[0])
	case *int:
		*v, err = parseInt(values[0])
	case *float64:
		*v, err = parseFloat(values[0])
	case *specid.ID:
		*v, err = parseID(values[0])
	case *bool:
		*v, err = parseBool(values[0])
	case *time.Time:
		*v, err = parseTime(values[0])

		// *[]T
	case *[]string:
		for _, value := range values {
			parsed, err := parseStr(value)
			if err != nil {
				return err
			}
			*v = append(*v, parsed)
		}
	case *[]int:
		for _, value := range values {
			parsed, err := parseInt(value)
			if err != nil {
				return err
			}
			*v = append(*v, parsed)
		}
	case *[]float64:
		for _, value := range values {
			parsed, err := parseFloat(value)
			if err != nil {
				return err
			}
			*v = append(*v, parsed)
		}
	case *[]specid.ID:
		for _, value := range values {
			parsed, err := parseID(value)
			if err != nil {
				return err
			}
			*v = append(*v, parsed)
		}
	case *[]bool:
		for _, value := range values {
			parsed, err := parseBool(value)
			if err != nil {
				return err
			}
			*v = append(*v, parsed)
		}
	case *[]time.Time:
		for _, value := range values {
			parsed, err := parseTime(value)
			if err != nil {
				return err
			}
			*v = append(*v, parsed)
		}
	}
	return err
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

// str {get, get first, get or, get first or}

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

// []str {get, get first, get or, get first or}

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

// int {get, get first, get or, get first or}

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

// []int {get, get first, get or, get first or}

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

// float {get, get first, get or, get first or}

func (p Params) GetFloat(key string) (float64, error) {
	var ret float64
	return ret, parse(p.get(key), &ret)
}

func (p Params) GetFirstFloat(keys ...string) (float64, error) {
	var ret float64
	return ret, parse(p.getFirst(keys), &ret)
}

func (p Params) GetOrFloat(key string, or float64) float64 {
	var ret float64
	if err := parse(p.get(key), &ret); err == nil {
		return ret
	}
	return or
}

func (p Params) GetFirstOrFloat(or float64, keys ...string) float64 {
	var ret float64
	if err := parse(p.getFirst(keys), &ret); err == nil {
		return ret
	}
	return or
}

// []float {get, get first, get or, get first or}

func (p Params) GetFloatList(key string) ([]float64, error) {
	var ret []float64
	return ret, parse(p.get(key), &ret)
}

func (p Params) GetFirstFloatList(keys ...string) ([]float64, error) {
	var ret []float64
	return ret, parse(p.getFirst(keys), &ret)
}

func (p Params) GetOrFloatList(key string, or []float64) []float64 {
	var ret []float64
	if err := parse(p.get(key), &ret); err == nil {
		return ret
	}
	return or
}

func (p Params) GetFirstOrFloatList(or []float64, keys ...string) []float64 {
	var ret []float64
	if err := parse(p.getFirst(keys), &ret); err == nil {
		return ret
	}
	return or
}

// specid.ID {get, get first, get or, get first or}

func (p Params) GetID(key string) (specid.ID, error) {
	var ret specid.ID
	return ret, parse(p.get(key), &ret)
}

func (p Params) GetFirstID(keys ...string) (specid.ID, error) {
	var ret specid.ID
	return ret, parse(p.getFirst(keys), &ret)
}

func (p Params) GetOrID(key string, or specid.ID) specid.ID {
	var ret specid.ID
	if err := parse(p.get(key), &ret); err == nil {
		return ret
	}
	return or
}

func (p Params) GetFirstOrID(or specid.ID, keys ...string) specid.ID {
	var ret specid.ID
	if err := parse(p.getFirst(keys), &ret); err == nil {
		return ret
	}
	return or
}

// []specid.ID {get, get first, get or, get first or}

func (p Params) GetIDList(key string) ([]specid.ID, error) {
	var ret []specid.ID
	return ret, parse(p.get(key), &ret)
}

func (p Params) GetFirstIDList(keys ...string) ([]specid.ID, error) {
	var ret []specid.ID
	return ret, parse(p.getFirst(keys), &ret)
}

func (p Params) GetOrIDList(key string, or []specid.ID) []specid.ID {
	var ret []specid.ID
	if err := parse(p.get(key), &ret); err == nil {
		return ret
	}
	return or
}

func (p Params) GetFirstOrIDList(or []specid.ID, keys ...string) []specid.ID {
	var ret []specid.ID
	if err := parse(p.getFirst(keys), &ret); err == nil {
		return ret
	}
	return or
}

// bool {get, get first, get or, get first or}

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

// []bool {get, get first, get or, get first or}

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

// time {get, get first, get or, get first or}

func (p Params) GetTime(key string) (time.Time, error) {
	var ret time.Time
	return ret, parse(p.get(key), &ret)
}

func (p Params) GetFirstTime(keys ...string) (time.Time, error) {
	var ret time.Time
	return ret, parse(p.getFirst(keys), &ret)
}

func (p Params) GetOrTime(key string, or time.Time) time.Time {
	var ret time.Time
	if err := parse(p.get(key), &ret); err == nil {
		return ret
	}
	return or
}

func (p Params) GetFirstOrTime(or time.Time, keys ...string) time.Time {
	var ret time.Time
	if err := parse(p.getFirst(keys), &ret); err == nil {
		return ret
	}
	return or
}
