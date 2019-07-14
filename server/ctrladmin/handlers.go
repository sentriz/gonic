package ctrladmin

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/sessions"

	"senan.xyz/g/gonic/model"
	"senan.xyz/g/gonic/scanner"
	"senan.xyz/g/gonic/server/key"
	"senan.xyz/g/gonic/server/lastfm"
)

func (c *Controller) ServeLogin(w http.ResponseWriter, r *http.Request) *response {
	return &response{template: "login.tmpl"}
}

func (c *Controller) ServeLoginDo(w http.ResponseWriter, r *http.Request) *response {
	session := r.Context().Value(key.Session).(*sessions.Session)
	username := r.FormValue("username")
	password := r.FormValue("password")
	if username == "" || password == "" {
		sessAddFlashW("please provide both a username and password", session)
		sessLogSave(w, r, session)
		return &response{redirect: r.Referer()}
	}
	user := c.DB.GetUserFromName(username)
	if user == nil || password != user.Password {
		sessAddFlashW("invalid username / password", session)
		sessLogSave(w, r, session)
		return &response{redirect: r.Referer()}
	}
	// put the user name into the session. future endpoints after this one
	// are wrapped with WithUserSession() which will get the name from the
	// session and put the row into the request context
	session.Values["user"] = user.Name
	sessLogSave(w, r, session)
	return &response{redirect: "/admin/home"}
}

func (c *Controller) ServeLogout(w http.ResponseWriter, r *http.Request) *response {
	session := r.Context().Value(key.Session).(*sessions.Session)
	session.Options.MaxAge = -1
	sessLogSave(w, r, session)
	return &response{redirect: "/admin/login"}
}

func (c *Controller) ServeHome(w http.ResponseWriter, r *http.Request) *response {
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
		Order("modified_at DESC").
		Limit(8).
		Find(&data.RecentFolders)
	data.IsScanning = scanner.IsScanning()
	if tStr := c.DB.GetSetting("last_scan_time"); tStr != "" {
		i, _ := strconv.ParseInt(tStr, 10, 64)
		data.LastScanTime = time.Unix(i, 0)
	}
	//
	return &response{
		template: "home.tmpl",
		data:     data,
	}
}

func (c *Controller) ServeChangeOwnPassword(w http.ResponseWriter, r *http.Request) *response {
	return &response{template: "change_own_password.tmpl"}
}

func (c *Controller) ServeChangeOwnPasswordDo(w http.ResponseWriter, r *http.Request) *response {
	session := r.Context().Value(key.Session).(*sessions.Session)
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	err := validatePasswords(passwordOne, passwordTwo)
	if err != nil {
		sessAddFlashW(err.Error(), session)
		sessLogSave(w, r, session)
		return &response{redirect: r.Referer()}
	}
	user := r.Context().Value(key.User).(*model.User)
	user.Password = passwordOne
	c.DB.Save(user)
	return &response{redirect: "/admin/home"}
}

func (c *Controller) ServeLinkLastFMDo(w http.ResponseWriter, r *http.Request) *response {
	token := r.URL.Query().Get("token")
	if token == "" {
		return &response{
			err:  "please provide a token",
			code: 400,
		}
	}
	sessionKey, err := lastfm.GetSession(
		c.DB.GetSetting("lastfm_api_key"),
		c.DB.GetSetting("lastfm_secret"),
		token,
	)
	if err != nil {
		session := r.Context().Value(key.Session).(*sessions.Session)
		sessAddFlashW(err.Error(), session)
		sessLogSave(w, r, session)
		return &response{redirect: "/admin/home"}
	}
	user := r.Context().Value(key.User).(*model.User)
	user.LastFMSession = sessionKey
	c.DB.Save(&user)
	return &response{redirect: "/admin/home"}
}

func (c *Controller) ServeUnlinkLastFMDo(w http.ResponseWriter, r *http.Request) *response {
	user := r.Context().Value(key.User).(*model.User)
	user.LastFMSession = ""
	c.DB.Save(&user)
	return &response{redirect: "/admin/home"}
}

func (c *Controller) ServeChangePassword(w http.ResponseWriter, r *http.Request) *response {
	username := r.URL.Query().Get("user")
	if username == "" {
		return &response{
			err:  "please provide a username",
			code: 400,
		}
	}
	user := c.DB.GetUserFromName(username)
	if user == nil {
		return &response{
			err:  "couldn't find a user with that name",
			code: 400,
		}
	}
	data := &templateData{}
	data.SelectedUser = user
	return &response{
		template: "change_own_password.tmpl",
		data:     data,
	}
}

func (c *Controller) ServeChangePasswordDo(w http.ResponseWriter, r *http.Request) *response {
	session := r.Context().Value(key.Session).(*sessions.Session)
	username := r.URL.Query().Get("user")
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	err := validatePasswords(passwordOne, passwordTwo)
	if err != nil {
		sessAddFlashW(err.Error(), session)
		sessLogSave(w, r, session)
		return &response{redirect: r.Referer()}
	}
	user := c.DB.GetUserFromName(username)
	user.Password = passwordOne
	c.DB.Save(user)
	return &response{redirect: "/admin/home"}
}

func (c *Controller) ServeDeleteUser(w http.ResponseWriter, r *http.Request) *response {
	username := r.URL.Query().Get("user")
	if username == "" {
		return &response{
			err:  "please provide a username",
			code: 400,
		}
	}
	user := c.DB.GetUserFromName(username)
	if user == nil {
		return &response{
			err:  "couldn't find a user with that name",
			code: 400,
		}
	}
	data := &templateData{}
	data.SelectedUser = user
	return &response{
		template: "delete_user.tmpl",
		data:     data,
	}
}

func (c *Controller) ServeDeleteUserDo(w http.ResponseWriter, r *http.Request) *response {
	username := r.URL.Query().Get("user")
	user := c.DB.GetUserFromName(username)
	c.DB.Delete(user)
	return &response{redirect: "/admin/home"}
}

func (c *Controller) ServeCreateUser(w http.ResponseWriter, r *http.Request) *response {
	return &response{template: "create_user.tmpl"}
}

func (c *Controller) ServeCreateUserDo(w http.ResponseWriter, r *http.Request) *response {
	session := r.Context().Value(key.Session).(*sessions.Session)
	username := r.FormValue("username")
	err := validateUsername(username)
	if err != nil {
		sessAddFlashW(err.Error(), session)
		sessLogSave(w, r, session)
		return &response{redirect: r.Referer()}
	}
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	err = validatePasswords(passwordOne, passwordTwo)
	if err != nil {
		sessAddFlashW(err.Error(), session)
		sessLogSave(w, r, session)
		return &response{redirect: r.Referer()}
	}
	user := model.User{
		Name:     username,
		Password: passwordOne,
	}
	err = c.DB.Create(&user).Error
	if err != nil {
		sessAddFlashWf("could not create user `%s`: %v", session, username, err)
		sessLogSave(w, r, session)
		return &response{redirect: r.Referer()}
	}
	return &response{redirect: "/admin/home"}
}

func (c *Controller) ServeUpdateLastFMAPIKey(w http.ResponseWriter, r *http.Request) *response {
	data := &templateData{}
	data.CurrentLastFMAPIKey = c.DB.GetSetting("lastfm_api_key")
	data.CurrentLastFMAPISecret = c.DB.GetSetting("lastfm_secret")
	return &response{
		template: "create_user.tmpl",
		data:     data,
	}
}

func (c *Controller) ServeUpdateLastFMAPIKeyDo(w http.ResponseWriter, r *http.Request) *response {
	session := r.Context().Value(key.Session).(*sessions.Session)
	apiKey := r.FormValue("api_key")
	secret := r.FormValue("secret")
	err := validateAPIKey(apiKey, secret)
	if err != nil {
		sessAddFlashW(err.Error(), session)
		sessLogSave(w, r, session)
		return &response{redirect: r.Referer()}
	}
	c.DB.SetSetting("lastfm_api_key", apiKey)
	c.DB.SetSetting("lastfm_secret", secret)
	return &response{redirect: r.Referer()}
}

func (c *Controller) ServeStartScanDo(w http.ResponseWriter, r *http.Request) *response {
	defer func() {
		go func() {
			err := scanner.
				New(c.DB, c.MusicPath).
				Start()
			if err != nil {
				log.Printf("error while scanning: %v\n", err)
			}
		}()
	}()
	session := r.Context().Value(key.Session).(*sessions.Session)
	sessAddFlashN("scan started. refresh for results", session)
	sessLogSave(w, r, session)
	return &response{redirect: "/admin/home"}
}
