package handler

import (
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/handler/utilities"
	"github.com/sentriz/gonic/lastfm"
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
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	user := c.GetUserFromName(username)
	if !(username == user.Name && password == user.Password) {
		session.AddFlash("invalid username / password")
		session.Save(r, w)
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	// put the user name into the session. future endpoints after this one
	// are wrapped with WithUserSession() which will get the name from the
	// session and put the row into the request context.
	session.Values["user"] = user.Name
	session.Save(r, w)
	http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
}

func (c *Controller) ServeLogout(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*sessions.Session)
	delete(session.Values, "user")
	session.Save(r, w)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

func (c *Controller) ServeHome(w http.ResponseWriter, r *http.Request) {
	var data templateData
	c.DB.Table("album_artists").Count(&data.ArtistCount)
	c.DB.Table("albums").Count(&data.AlbumCount)
	c.DB.Table("tracks").Count(&data.TrackCount)
	c.DB.Find(&data.AllUsers)
	data.CurrentLastFMAPIKey = c.GetSetting("lastfm_api_key")
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
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	user := r.Context().Value("user").(*db.User)
	user.Password = passwordOne
	c.DB.Save(user)
	http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
}

func (c *Controller) ServeLinkLastFMDo(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "please provide a token", 400)
		return
	}
	sessionKey, err := lastfm.GetSession(
		c.GetSetting("lastfm_api_key"),
		c.GetSetting("lastfm_secret"),
		token,
	)
	session := r.Context().Value("session").(*sessions.Session)
	if err != nil {
		session.AddFlash(err.Error())
		session.Save(r, w)
		http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
		return
	}
	user := r.Context().Value("user").(*db.User)
	user.LastFMSession = sessionKey
	c.DB.Save(&user)
	http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
}

func (c *Controller) ServeUnlinkLastFMDo(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*db.User)
	user.LastFMSession = ""
	c.DB.Save(&user)
	http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
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
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	user.Password = passwordOne
	c.DB.Save(&user)
	http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
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
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	err = utilities.ValidatePasswords(passwordOne, passwordTwo)
	if err != nil {
		session.AddFlash(err.Error())
		session.Save(r, w)
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
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
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
}

func (c *Controller) ServeUpdateLastFMAPIKey(w http.ResponseWriter, r *http.Request) {
	var data templateData
	data.CurrentLastFMAPIKey = c.GetSetting("lastfm_api_key")
	data.CurrentLastFMAPISecret = c.GetSetting("lastfm_secret")
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
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	c.SetSetting("lastfm_api_key", apiKey)
	c.SetSetting("lastfm_secret", secret)
	http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
}
