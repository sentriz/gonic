package handler

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gorilla/sessions"
	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/handler/utilities"
	"github.com/sentriz/gonic/subsonic"

	"github.com/jinzhu/gorm"
	"github.com/wader/gormstore"
)

var (
	templates = make(map[string]*template.Template)
)

func init() {
	templates["login"] = template.Must(template.ParseFiles(
		filepath.Join("templates", "layout.tmpl"),
		filepath.Join("templates", "pages", "login.tmpl"),
	))
	templates["home"] = template.Must(template.ParseFiles(
		filepath.Join("templates", "layout.tmpl"),
		filepath.Join("templates", "user.tmpl"),
		filepath.Join("templates", "pages", "home.tmpl"),
	))
	templates["change_password"] = template.Must(template.ParseFiles(
		filepath.Join("templates", "layout.tmpl"),
		filepath.Join("templates", "user.tmpl"),
		filepath.Join("templates", "pages", "change_password.tmpl"),
	))
	templates["change_own_password"] = template.Must(template.ParseFiles(
		filepath.Join("templates", "layout.tmpl"),
		filepath.Join("templates", "user.tmpl"),
		filepath.Join("templates", "pages", "change_own_password.tmpl"),
	))
	templates["create_user"] = template.Must(template.ParseFiles(
		filepath.Join("templates", "layout.tmpl"),
		filepath.Join("templates", "user.tmpl"),
		filepath.Join("templates", "pages", "create_user.tmpl"),
	))
	templates["update_lastfm_api_key"] = template.Must(template.ParseFiles(
		filepath.Join("templates", "layout.tmpl"),
		filepath.Join("templates", "user.tmpl"),
		filepath.Join("templates", "pages", "update_lastfm_api_key.tmpl"),
	))
}

type Controller struct {
	DB     *gorm.DB
	SStore *gormstore.Store
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
	ArtistCount            uint
	AlbumCount             uint
	TrackCount             uint
	CurrentLastFMAPIKey    string
	CurrentLastFMAPISecret string
	RequestRoot            string
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
	code uint64, message string) {
	respondRaw(w, r, http.StatusBadRequest, subsonic.NewError(
		code, message,
	))
}

func renderTemplate(w http.ResponseWriter, r *http.Request,
	name string, data *templateData) {
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
	scheme := utilities.FirstExisting(
		"http", // fallback
		r.Header.Get("X-Forwarded-Proto"),
		r.Header.Get("X-Forwarded-Scheme"),
		r.URL.Scheme,
	)
	host := utilities.FirstExisting(
		"localhost:7373", // fallback
		r.Header.Get("X-Forwarded-Host"),
		r.Host,
	)
	data.RequestRoot = fmt.Sprintf("%s://%s", scheme, host)
	err := templates[name].ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, fmt.Sprintf("500 when executing: %v", err), 500)
		return
	}
}
