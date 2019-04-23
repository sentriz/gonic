package handler

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
	"github.com/wader/gormstore"

	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/subsonic"
)

type Controller struct {
	DB        *gorm.DB
	SStore    *gormstore.Store
	Templates map[string]*template.Template
}

func (c *Controller) GetSetting(key string) string {
	var setting db.Setting
	c.DB.Where("key = ?", key).First(&setting)
	return setting.Value
}

func (c *Controller) SetSetting(key, value string) {
	c.DB.
		Where(db.Setting{Key: key}).
		Assign(db.Setting{Value: value}).
		FirstOrCreate(&db.Setting{})
}

func (c *Controller) GetUserFromName(name string) *db.User {
	var user db.User
	c.DB.Where("name = ?", name).First(&user)
	return &user
}

type templateData struct {
	Flashes                []interface{}
	User                   *db.User
	SelectedUser           *db.User
	AllUsers               []*db.User
	ArtistCount            int
	AlbumCount             int
	TrackCount             int
	CurrentLastFMAPIKey    string
	CurrentLastFMAPISecret string
	RequestRoot            string
}

func getStrParam(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

func getStrParamOr(r *http.Request, key, or string) string {
	val := getStrParam(r, key)
	if val == "" {
		return or
	}
	return val
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

func respondRaw(w http.ResponseWriter, r *http.Request,
	code int, sub *subsonic.Response) {
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

func respond(w http.ResponseWriter, r *http.Request,
	sub *subsonic.Response) {
	respondRaw(w, r, http.StatusOK, sub)
}

func respondError(w http.ResponseWriter, r *http.Request,
	code int, message string) {
	respondRaw(w, r, http.StatusBadRequest, subsonic.NewError(
		code, message,
	))
}

func renderTemplate(w http.ResponseWriter, r *http.Request,
	tmpl *template.Template, data *templateData) {
	session := r.Context().Value("session").(*sessions.Session)
	if data == nil {
		data = &templateData{}
	}
	data.Flashes = session.Flashes()
	session.Save(r, w)
	user, ok := r.Context().Value("user").(*db.User)
	if ok {
		data.User = user
	}
	err := tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("500 when executing: %v", err), 500)
		return
	}
}
