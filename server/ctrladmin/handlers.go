package ctrladmin

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/mmcdole/gofeed"

	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/server/encode"
	"go.senan.xyz/gonic/server/scanner"
	"go.senan.xyz/gonic/server/scrobble/lastfm"
	"go.senan.xyz/gonic/server/scrobble/listenbrainz"
)

func firstExisting(or string, strings ...string) string {
	for _, s := range strings {
		if s != "" {
			return s
		}
	}
	return or
}

func doScan(scanner *scanner.Scanner, opts scanner.ScanOptions) {
	go func() {
		if err := scanner.Start(opts); err != nil {
			log.Printf("error while scanning: %v\n", err)
		}
	}()
}

func (c *Controller) ServeNotFound(r *http.Request) *Response {
	return &Response{template: "not_found.tmpl", code: 404}
}

func (c *Controller) ServeLogin(r *http.Request) *Response {
	return &Response{template: "login.tmpl"}
}

func (c *Controller) ServeHome(r *http.Request) *Response {
	data := &templateData{}
	// ** begin stats box
	c.DB.Table("artists").Count(&data.ArtistCount)
	c.DB.Table("albums").Count(&data.AlbumCount)
	c.DB.Table("tracks").Count(&data.TrackCount)
	// ** begin lastfm box
	scheme := firstExisting(
		"http", // fallback
		r.Header.Get("X-Forwarded-Proto"),
		r.Header.Get("X-Forwarded-Scheme"),
		r.URL.Scheme,
	)
	host := firstExisting(
		"localhost:4747", // fallback
		r.Header.Get("X-Forwarded-Host"),
		r.Host,
	)
	data.RequestRoot = fmt.Sprintf("%s://%s", scheme, host)
	data.CurrentLastFMAPIKey = c.DB.GetSetting("lastfm_api_key")
	data.DefaultListenBrainzURL = listenbrainz.BaseURL
	// ** begin users box
	c.DB.Find(&data.AllUsers)
	// ** begin recent folders box
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
	user := r.Context().Value(CtxUser).(*db.User)
	// ** begin playlists box
	c.DB.
		Where("user_id=?", user.ID).
		Limit(20).
		Find(&data.Playlists)
	// ** begin transcoding box
	c.DB.
		Where("user_id=?", user.ID).
		Find(&data.TranscodePreferences)
	for profile := range encode.Profiles() {
		data.TranscodeProfiles = append(data.TranscodeProfiles, profile)
	}
	// ** begin podcasts box
	c.DB.Find(&data.Podcasts)
	//
	return &Response{
		template: "home.tmpl",
		data:     data,
	}
}

func (c *Controller) ServeChangeOwnUsername(r *http.Request) *Response {
	return &Response{template: "change_own_username.tmpl"}
}

func (c *Controller) ServeChangeOwnUsernameDo(r *http.Request) *Response {
	username := r.FormValue("username")
	if err := validateUsername(username); err != nil {
		return &Response{
			redirect: r.Referer(),
			flashW:   []string{err.Error()},
		}
	}
	user := r.Context().Value(CtxUser).(*db.User)
	user.Name = username
	c.DB.Save(user)
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeChangeOwnPassword(r *http.Request) *Response {
	return &Response{template: "change_own_password.tmpl"}
}

func (c *Controller) ServeChangeOwnPasswordDo(r *http.Request) *Response {
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	if err := validatePasswords(passwordOne, passwordTwo); err != nil {
		return &Response{
			redirect: r.Referer(),
			flashW:   []string{err.Error()},
		}
	}
	user := r.Context().Value(CtxUser).(*db.User)
	user.Password = passwordOne
	c.DB.Save(user)
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeLinkLastFMDo(r *http.Request) *Response {
	token := r.URL.Query().Get("token")
	if token == "" {
		return &Response{
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
		return &Response{
			redirect: "/admin/home",
			flashW:   []string{err.Error()},
		}
	}
	user := r.Context().Value(CtxUser).(*db.User)
	user.LastFMSession = sessionKey
	c.DB.Save(&user)
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeUnlinkLastFMDo(r *http.Request) *Response {
	user := r.Context().Value(CtxUser).(*db.User)
	user.LastFMSession = ""
	c.DB.Save(&user)
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeLinkListenBrainzDo(r *http.Request) *Response {
	token := r.FormValue("token")
	url := r.FormValue("url")
	if token == "" || url == "" {
		return &Response{
			redirect: r.Referer(),
			flashW:   []string{"please provide a url and token"},
		}
	}
	user := r.Context().Value(CtxUser).(*db.User)
	user.ListenBrainzURL = url
	user.ListenBrainzToken = token
	c.DB.Save(&user)
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeUnlinkListenBrainzDo(r *http.Request) *Response {
	user := r.Context().Value(CtxUser).(*db.User)
	user.ListenBrainzURL = ""
	user.ListenBrainzToken = ""
	c.DB.Save(&user)
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeChangeUsername(r *http.Request) *Response {
	username := r.URL.Query().Get("user")
	if username == "" {
		return &Response{
			err:  "please provide a username",
			code: 400,
		}
	}
	user := c.DB.GetUserByName(username)
	if user == nil {
		return &Response{
			err:  "couldn't find a user with that name",
			code: 400,
		}
	}
	data := &templateData{}
	data.SelectedUser = user
	return &Response{
		template: "change_username.tmpl",
		data:     data,
	}
}

func (c *Controller) ServeChangeUsernameDo(r *http.Request) *Response {
	username := r.URL.Query().Get("user")
	usernameNew := r.FormValue("username")
	if err := validateUsername(usernameNew); err != nil {
		return &Response{
			redirect: r.Referer(),
			flashW:   []string{err.Error()},
		}
	}
	user := c.DB.GetUserByName(username)
	user.Name = usernameNew
	c.DB.Save(user)
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeChangePassword(r *http.Request) *Response {
	username := r.URL.Query().Get("user")
	if username == "" {
		return &Response{
			err:  "please provide a username",
			code: 400,
		}
	}
	user := c.DB.GetUserByName(username)
	if user == nil {
		return &Response{
			err:  "couldn't find a user with that name",
			code: 400,
		}
	}
	data := &templateData{}
	data.SelectedUser = user
	return &Response{
		template: "change_password.tmpl",
		data:     data,
	}
}

func (c *Controller) ServeChangePasswordDo(r *http.Request) *Response {
	username := r.URL.Query().Get("user")
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	if err := validatePasswords(passwordOne, passwordTwo); err != nil {
		return &Response{
			redirect: r.Referer(),
			flashW:   []string{err.Error()},
		}
	}
	user := c.DB.GetUserByName(username)
	user.Password = passwordOne
	c.DB.Save(user)
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeDeleteUser(r *http.Request) *Response {
	username := r.URL.Query().Get("user")
	if username == "" {
		return &Response{
			err:  "please provide a username",
			code: 400,
		}
	}
	user := c.DB.GetUserByName(username)
	if user == nil {
		return &Response{
			err:  "couldn't find a user with that name",
			code: 400,
		}
	}
	data := &templateData{}
	data.SelectedUser = user
	return &Response{
		template: "delete_user.tmpl",
		data:     data,
	}
}

func (c *Controller) ServeDeleteUserDo(r *http.Request) *Response {
	username := r.URL.Query().Get("user")
	user := c.DB.GetUserByName(username)
	if user.IsAdmin {
		return &Response{
			redirect: "/admin/home",
			flashW:   []string{"can't delete the admin user"},
		}
	}
	c.DB.Delete(user)
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeCreateUser(r *http.Request) *Response {
	return &Response{template: "create_user.tmpl"}
}

func (c *Controller) ServeCreateUserDo(r *http.Request) *Response {
	username := r.FormValue("username")
	err := validateUsername(username)
	if err != nil {
		return &Response{
			redirect: r.Referer(),
			flashW:   []string{err.Error()},
		}
	}
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	err = validatePasswords(passwordOne, passwordTwo)
	if err != nil {
		return &Response{
			redirect: r.Referer(),
			flashW:   []string{err.Error()},
		}
	}
	user := db.User{
		Name:     username,
		Password: passwordOne,
	}
	if err := c.DB.Create(&user).Error; err != nil {
		return &Response{
			redirect: r.Referer(),
			flashW:   []string{fmt.Sprintf("could not create user `%s`: %v", username, err)},
		}
	}
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeUpdateLastFMAPIKey(r *http.Request) *Response {
	data := &templateData{}
	data.CurrentLastFMAPIKey = c.DB.GetSetting("lastfm_api_key")
	data.CurrentLastFMAPISecret = c.DB.GetSetting("lastfm_secret")
	return &Response{
		template: "update_lastfm_api_key.tmpl",
		data:     data,
	}
}

func (c *Controller) ServeUpdateLastFMAPIKeyDo(r *http.Request) *Response {
	apiKey := r.FormValue("api_key")
	secret := r.FormValue("secret")
	if err := validateAPIKey(apiKey, secret); err != nil {
		return &Response{
			redirect: r.Referer(),
			flashW:   []string{err.Error()},
		}
	}
	c.DB.SetSetting("lastfm_api_key", apiKey)
	c.DB.SetSetting("lastfm_secret", secret)
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeStartScanIncDo(r *http.Request) *Response {
	defer doScan(c.Scanner, scanner.ScanOptions{})
	return &Response{
		redirect: "/admin/home",
		flashN:   []string{"incremental scan started. refresh for results"},
	}
}

func (c *Controller) ServeStartScanFullDo(r *http.Request) *Response {
	defer doScan(c.Scanner, scanner.ScanOptions{IsFull: true})
	return &Response{
		redirect: "/admin/home",
		flashN:   []string{"full scan started. refresh for results"},
	}
}

func (c *Controller) ServeCreateTranscodePrefDo(r *http.Request) *Response {
	client := r.FormValue("client")
	profile := r.FormValue("profile")
	if client == "" {
		return &Response{
			redirect: "/admin/home",
			flashW:   []string{"please provide a client name"},
		}
	}
	user := r.Context().Value(CtxUser).(*db.User)
	pref := db.TranscodePreference{
		UserID:  user.ID,
		Client:  client,
		Profile: profile,
	}
	if err := c.DB.Create(&pref).Error; err != nil {
		return &Response{
			redirect: "/admin/home",
			flashW:   []string{fmt.Sprintf("could not create preference: %v", err)},
		}
	}
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeDeleteTranscodePrefDo(r *http.Request) *Response {
	user := r.Context().Value(CtxUser).(*db.User)
	client := r.URL.Query().Get("client")
	if client == "" {
		return &Response{
			err:  "please provide a client",
			code: 400,
		}
	}
	c.DB.
		Where("user_id=? AND client=?", user.ID, client).
		Delete(db.TranscodePreference{})
	return &Response{
		redirect: "/admin/home",
	}
}

func (c *Controller) ServePodcastAddDo(r *http.Request) *Response {
	user := r.Context().Value(CtxUser).(*db.User)
	rssURL := r.FormValue("feed")
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(rssURL)
	if err != nil {
		return &Response{
			redirect: "/admin/home",
			flashW:   []string{fmt.Sprintf("could not create feed: %v", err)},
		}
	}
	if _, err = c.Podcasts.AddNewPodcast(feed, user.ID); err != nil {
		return &Response{
			redirect: "/admin/home",
			flashW:   []string{fmt.Sprintf("could not create feed: %v", err)},
		}
	}
	return &Response{
		redirect: "/admin/home",
	}
}

func (c *Controller) ServePodcastDeleteDo(r *http.Request) *Response {
	user := r.Context().Value(CtxUser).(*db.User)
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		return &Response{
			err:  "please provide a valid podcast id",
			code: 400,
		}
	}
	err = c.Podcasts.DeletePodcast(user.ID, id)
	if err != nil {
		return &Response{
			err:  "please provide a valid podcast id",
			code: 400,
		}
	}
	return &Response{
		redirect: "/admin/home",
	}
}
