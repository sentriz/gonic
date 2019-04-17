package handler

import (
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/handler/utilities"
)

func (c *Controller) ServeLogin(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, r, "login", nil)
}

func (c *Controller) ServeLoginDo(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*sessions.Session)
	username := r.FormValue("username")
	password := r.FormValue("password")
	if username == "" || password == "" {
		session.AddFlash("please provide both a username and password")
		session.Save(r, w)
		http.Redirect(w, r, r.Header.Get("Referer"), 302)
		return
	}
	var user db.User
	c.DB.Where("name = ?", username).First(&user)
	if !(username == user.Name && password == user.Password) {
		session.AddFlash("invalid username / password")
		session.Save(r, w)
		http.Redirect(w, r, r.Header.Get("Referer"), 302)
		return
	}
	session.Values["user"] = user
	session.Save(r, w)
	http.Redirect(w, r, "/admin/home", 303)
}

func (c *Controller) ServeLogout(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*sessions.Session)
	delete(session.Values, "user")
	session.Save(r, w)
	http.Redirect(w, r, "/admin/login", 303)
}

func (c *Controller) ServeHome(w http.ResponseWriter, r *http.Request) {
	var data templateData
	c.DB.Table("album_artists").Count(&data.ArtistCount)
	c.DB.Table("albums").Count(&data.AlbumCount)
	c.DB.Table("tracks").Count(&data.TrackCount)
	c.DB.Find(&data.AllUsers)
	var apiKey db.Setting
	c.DB.Where("key = ?", "lastfm_api_key").First(&apiKey)
	data.CurrentLastFMAPIKey = apiKey.Value
	renderTemplate(w, r, "home", &data)
}

func (c *Controller) ServeChangeOwnPassword(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, r, "change_own_password", nil)
}

func (c *Controller) ServeChangeOwnPasswordDo(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*sessions.Session)
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	err := utilities.ValidatePasswords(passwordOne, passwordTwo)
	if err != nil {
		session.AddFlash(err.Error())
		session.Save(r, w)
		http.Redirect(w, r, r.Header.Get("Referer"), 302)
		return
	}
	user, _ := session.Values["user"].(*db.User)
	user.Password = passwordOne
	c.DB.Save(user)
	http.Redirect(w, r, "/admin/home", 303)
}

func (c *Controller) ServeLinkLastFMCallback(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "please provide a token", 400)
		return
	}
	_ = token
}

func (c *Controller) ServeChangePassword(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("user")
	if username == "" {
		http.Error(w, "please provide a username", 400)
		return
	}
	var user db.User
	err := c.DB.Where("name = ?", username).First(&user).Error
	if gorm.IsRecordNotFoundError(err) {
		http.Error(w, "couldn't find a user with that name", 400)
		return
	}
	var data templateData
	data.SelectedUser = &user
	renderTemplate(w, r, "change_password", &data)
}

func (c *Controller) ServeChangePasswordDo(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*sessions.Session)
	username := r.URL.Query().Get("user")
	var user db.User
	c.DB.Where("name = ?", username).First(&user)
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	err := utilities.ValidatePasswords(passwordOne, passwordTwo)
	if err != nil {
		session.AddFlash(err.Error())
		session.Save(r, w)
		http.Redirect(w, r, r.Header.Get("Referer"), 302)
		return
	}
	user.Password = passwordOne
	c.DB.Save(&user)
	http.Redirect(w, r, "/admin/home", 303)
}

func (c *Controller) ServeCreateUser(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, r, "create_user", nil)
}

func (c *Controller) ServeCreateUserDo(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*sessions.Session)
	username := r.FormValue("username")
	err := utilities.ValidateUsername(username)
	if err != nil {
		session.AddFlash(err.Error())
		session.Save(r, w)
		http.Redirect(w, r, r.Header.Get("Referer"), 302)
		return
	}
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	err = utilities.ValidatePasswords(passwordOne, passwordTwo)
	if err != nil {
		session.AddFlash(err.Error())
		session.Save(r, w)
		http.Redirect(w, r, r.Header.Get("Referer"), 302)
		return
	}
	user := db.User{
		Name:     username,
		Password: passwordOne,
	}
	err = c.DB.Create(&user).Error
	if err != nil {
		session.AddFlash(fmt.Sprintf(
			"could not create user `%s`: %v", username, err,
		))
		session.Save(r, w)
		http.Redirect(w, r, r.Header.Get("Referer"), 302)
		return
	}
	http.Redirect(w, r, "/admin/home", 303)
}

func (c *Controller) ServeUpdateLastFMAPIKey(w http.ResponseWriter, r *http.Request) {
	var data templateData
	var apiKey db.Setting
	var secret db.Setting
	c.DB.Where("key = ?", "lastfm_api_key").First(&apiKey)
	c.DB.Where("key = ?", "lastfm_secret").First(&secret)
	data.CurrentLastFMAPIKey = apiKey.Value
	data.CurrentLastFMAPISecret = secret.Value
	renderTemplate(w, r, "update_lastfm_api_key", &data)
}

func (c *Controller) ServeUpdateLastFMAPIKeyDo(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*sessions.Session)
	apiKey := r.FormValue("api_key")
	secret := r.FormValue("secret")
	err := utilities.ValidateAPIKey(apiKey, secret)
	if err != nil {
		session.AddFlash(err.Error())
		session.Save(r, w)
		http.Redirect(w, r, r.Header.Get("Referer"), 302)
		return
	}
	c.DB.
		Where(db.Setting{Key: "lastfm_api_key"}).
		Assign(db.Setting{Value: apiKey}).
		FirstOrCreate(&db.Setting{})
	c.DB.
		Where(db.Setting{Key: "lastfm_secret"}).
		Assign(db.Setting{Value: secret}).
		FirstOrCreate(&db.Setting{})
	http.Redirect(w, r, "/admin/home", 303)
}
