package params

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type Params struct {
	values url.Values
}

func New(r *http.Request) Params {
	// first load params from the url
	params := r.URL.Query()
	// also if there's any in the post body, use those too
	if err := r.ParseForm(); err != nil {
		return Params{params}
	}
	for k, v := range r.Form {
		params[k] = v
	}
	return Params{params}
}

func (p Params) Get(key string) string {
	return p.values.Get(key)
}

func (p Params) GetOr(key, or string) string {
	val := p.Get(key)
	if val == "" {
		return or
	}
	return val
}

func (p Params) GetInt(key string) (int, error) {
	strVal := p.values.Get(key)
	if strVal == "" {
		return 0, fmt.Errorf("no param with key `%s`", key)
	}
	val, err := strconv.Atoi(strVal)
	if err != nil {
		return 0, fmt.Errorf("not an int `%s`", strVal)
	}
	return val, nil
}

func (p Params) GetIntOr(key string, or int) int {
	val, err := p.GetInt(key)
	if err != nil {
		return or
	}
	return val
}

func (p Params) GetFirstList(keys ...string) []string {
	for _, key := range keys {
		if v, ok := p.values[key]; ok && len(v) > 0 {
			return v
		}
	}
	return nil
}

func (p Params) GetFirstListInt(keys ...string) []int {
	v := p.GetFirstList(keys...)
	if v == nil {
		return nil
	}
	ret := make([]int, 0, len(v))
	for _, p := range v {
		i, _ := strconv.Atoi(p)
		ret = append(ret, i)
	}
	return ret
}
