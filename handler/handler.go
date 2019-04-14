package handler

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/jinzhu/gorm"
	"github.com/sentriz/gonic/subsonic"
)

type Controller struct {
	DB *gorm.DB
}

func getStrParam(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

func getIntParam(r *http.Request, key string) (int, error) {
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

func getIntParamOr(r *http.Request, key string, or int) int {
	val, err := getIntParam(r, key)
	if err != nil {
		return or
	}
	return val
}

func respondRaw(w http.ResponseWriter, r *http.Request, code int, sub *subsonic.Response) {
	res := subsonic.MetaResponse{
		Response: sub,
	}
	switch r.URL.Query().Get("f") {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		data, err := json.Marshal(res)
		if err != nil {
			log.Printf("could not marshall to json: %v\n", err)
		}
		w.Write(data)
	case "jsonp":
		w.Header().Set("Content-Type", "application/javascript")
		data, err := json.Marshal(res)
		if err != nil {
			log.Printf("could not marshall to json: %v\n", err)
		}
		callback := r.URL.Query().Get("callback")
		w.Write([]byte(callback))
		w.Write([]byte("("))
		w.Write(data)
		w.Write([]byte(");"))
	default:
		w.Header().Set("Content-Type", "application/xml")
		data, err := xml.Marshal(res)
		if err != nil {
			log.Printf("could not marshall to xml: %v\n", err)
		}
		w.Write(data)
	}
}

func respond(w http.ResponseWriter, r *http.Request, sub *subsonic.Response) {
	respondRaw(w, r, http.StatusOK, sub)
}

func respondError(w http.ResponseWriter, r *http.Request, code uint64, message string) {
	respondRaw(w, r, http.StatusBadRequest, subsonic.NewError(
		code, message,
	))
}
