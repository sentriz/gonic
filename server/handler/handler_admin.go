package handler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"

	"senan.xyz/g/gonic/model"
	"senan.xyz/g/gonic/scanner"
	"senan.xyz/g/gonic/server/lastfm"
)

func (c *Controller) ServeLogin(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, r, c.Templates["login.tmpl"], nil)
}

func (c *Controller) ServeLoginDo(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(contextSessionKey).(*sessions.Session)
	username := r.FormValue("username")
	password := r.FormValue("password")
	if username == "" || password == "" {
		sessAddFlashW("please provide both a username and password", session)
		sessLogSave(w, r, session)
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	user := c.DB.GetUserFromName(username)
	if user == nil || password != user.Password {
		sessAddFlashW("invalid username / password", session)
		sessLogSave(w, r, session)
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	// put the user name into the session. future endpoints after this one
	// are wrapped with WithUserSession() which will get the name from the
	// session and put the row into the request context.
	session.Values["user"] = user.Name
	sessLogSave(w, r, session)
	http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
}

func (c *Controller) ServeLogout(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(contextSessionKey).(*sessions.Session)
	session.Options.MaxAge = -1
	sessLogSave(w, r, session)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

func (c *Controller) ServeHome(w http.ResponseWriter, r *http.Request) {
	data := &templateData{}
	//
	// stats box
	c.DB.Table("artists").Count(&data.ArtistCount)
	c.DB.Table("albums").Count(&data.AlbumCount)
	c.DB.Table("tracks").Count(&data.TrackCount)
	//
	// lastfm box
	scheme := firstExisting(
		"http", // fallback
		r.Header.Get("X-Forwarded-Proto"),
		r.Header.Get("X-Forwarded-Scheme"),
		r.URL.Scheme,
	)
	host := firstExisting(
		"localhost:7373", // fallback
		r.Header.Get("X-Forwarded-Host"),
		r.Host,
	)
	data.RequestRoot = fmt.Sprintf("%s://%s", scheme, host)
	data.CurrentLastFMAPIKey = c.DB.GetSetting("lastfm_api_key")
	//
	// users box
	c.DB.Find(&data.AllUsers)
	//
	// recent folders box
	c.DB.
		Where("tag_artist_id IS NOT NULL").
		Order("updated_at DESC").
		Limit(8).
		Find(&data.RecentFolders)
	data.IsScanning = scanner.IsScanning()
	if tStr := c.DB.GetSetting("last_scan_time"); tStr != "" {
		i, _ := strconv.ParseInt(tStr, 10, 64)
		data.LastScanTime = time.Unix(i, 0)
	}
	//
	renderTemplate(w, r, c.Templates["home.tmpl"], data)
}

func (c *Controller) ServeChangeOwnPassword(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, r, c.Templates["change_own_password.tmpl"], nil)
}

func (c *Controller) ServeChangeOwnPasswordDo(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(contextSessionKey).(*sessions.Session)
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	err := validatePasswords(passwordOne, passwordTwo)
	if err != nil {
		sessAddFlashW(err.Error(), session)
		sessLogSave(w, r, session)
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	user := r.Context().Value(contextUserKey).(*model.User)
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
		c.DB.GetSetting("lastfm_api_key"),
		c.DB.GetSetting("lastfm_secret"),
		token,
	)
	session := r.Context().Value(contextSessionKey).(*sessions.Session)
	if err != nil {
		sessAddFlashW(err.Error(), session)
		sessLogSave(w, r, session)
		http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
		return
	}
	user := r.Context().Value(contextUserKey).(*model.User)
	user.LastFMSession = sessionKey
	c.DB.Save(&user)
	http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
}

func (c *Controller) ServeUnlinkLastFMDo(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(contextUserKey).(*model.User)
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
	user := &model.User{}
	err := c.DB.
		Where("name = ?", username).
		First(user).
		Error
	if gorm.IsRecordNotFoundError(err) {
		http.Error(w, "couldn't find a user with that name", 400)
		return
	}
	data := &templateData{}
	data.SelectedUser = user
	renderTemplate(w, r, c.Templates["change_password.tmpl"], data)
}

func (c *Controller) ServeChangePasswordDo(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(contextSessionKey).(*sessions.Session)
	username := r.URL.Query().Get("user")
	user := &model.User{}
	c.DB.
		Where("name = ?", username).
		First(user)
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	err := validatePasswords(passwordOne, passwordTwo)
	if err != nil {
		sessAddFlashW(err.Error(), session)
		sessLogSave(w, r, session)
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	user.Password = passwordOne
	c.DB.Save(user)
	http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
}

func (c *Controller) ServeDeleteUser(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("user")
	if username == "" {
		http.Error(w, "please provide a username", 400)
		return
	}
	user := &model.User{}
	err := c.DB.
		Where("name = ?", username).
		First(user).
		Error
	if gorm.IsRecordNotFoundError(err) {
		http.Error(w, "couldn't find a user with that name", 400)
		return
	}
	data := &templateData{}
	data.SelectedUser = user
	renderTemplate(w, r, c.Templates["delete_user.tmpl"], data)
}

func (c *Controller) ServeDeleteUserDo(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("user")
	user := &model.User{}
	c.DB.
		Where("name = ?", username).
		First(user)
	c.DB.Delete(user)
	http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
}

func (c *Controller) ServeCreateUser(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, r, c.Templates["create_user.tmpl"], nil)
}

func (c *Controller) ServeCreateUserDo(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(contextSessionKey).(*sessions.Session)
	username := r.FormValue("username")
	err := validateUsername(username)
	if err != nil {
		sessAddFlashW(err.Error(), session)
		sessLogSave(w, r, session)
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	err = validatePasswords(passwordOne, passwordTwo)
	if err != nil {
		sessAddFlashW(err.Error(), session)
		sessLogSave(w, r, session)
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	user := model.User{
		Name:     username,
		Password: passwordOne,
	}
	err = c.DB.Create(&user).Error
	if err != nil {
		sessAddFlashW(fmt.Sprintf(
			"could not create user `%s`: %v", username, err,
		), session)
		sessLogSave(w, r, session)
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
}

func (c *Controller) ServeUpdateLastFMAPIKey(w http.ResponseWriter, r *http.Request) {
	data := &templateData{}
	data.CurrentLastFMAPIKey = c.DB.GetSetting("lastfm_api_key")
	data.CurrentLastFMAPISecret = c.DB.GetSetting("lastfm_secret")
	renderTemplate(w, r, c.Templates["update_lastfm_api_key.tmpl"], data)
}

func (c *Controller) ServeUpdateLastFMAPIKeyDo(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(contextSessionKey).(*sessions.Session)
	apiKey := r.FormValue("api_key")
	secret := r.FormValue("secret")
	err := validateAPIKey(apiKey, secret)
	if err != nil {
		sessAddFlashW(err.Error(), session)
		sessLogSave(w, r, session)
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	c.DB.SetSetting("lastfm_api_key", apiKey)
	c.DB.SetSetting("lastfm_secret", secret)
	http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
}

func (c *Controller) ServeStartScanDo(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(contextSessionKey).(*sessions.Session)
	sessAddFlashN("scan started", session)
	sessLogSave(w, r, session)
	http.Redirect(w, r, "/admin/home", http.StatusSeeOther)
	go func() {
		err := scanner.
			New(c.DB, c.MusicPath).
			Start()
		if err != nil {
			log.Printf("error while scanning: %v\n", err)
		}
	}()
}
