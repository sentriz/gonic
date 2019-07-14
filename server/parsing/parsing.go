package parsing

import (
	"fmt"
	"net/http"
	"strconv"
)

func GetStrParam(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

func GetStrParamOr(r *http.Request, key, or string) string {
	val := GetStrParam(r, key)
	if val == "" {
		return or
	}
	return val
}

func GetIntParam(r *http.Request, key string) (int, error) {
	strVal := r.URL.Query().Get(key)
	if strVal == "" {
		return 0, fmt.Errorf("no param with key `%s`", key)
	}
	val, err := strconv.Atoi(strVal)
	if err != nil {
		return 0, fmt.Errorf("not an int `%s`", strVal)
	}
	return val, nil
}

func GetIntParamOr(r *http.Request, key string, or int) int {
	val, err := GetIntParam(r, key)
	if err != nil {
		return or
	}
	return val
}
