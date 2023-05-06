//nolint:goerr113
package ctrladmin

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif" // to decode uploaded GIF avatars
	"image/jpeg"
	_ "image/png" // to decode uploaded PNG avatars
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/nfnt/resize"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scanner"
	"go.senan.xyz/gonic/scrobble/lastfm"
	"go.senan.xyz/gonic/scrobble/listenbrainz"
	"go.senan.xyz/gonic/transcode"
)

func doScan(scanner *scanner.Scanner, opts scanner.ScanOptions) {
	go func() {
		if _, err := scanner.ScanAndClean(opts); err != nil {
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
	user := r.Context().Value(CtxUser).(*db.User)

	data := &templateData{}
	// stats box
	c.DB.Model(&db.Artist{}).Count(&data.ArtistCount)
	c.DB.Model(&db.Album{}).Count(&data.AlbumCount)
	c.DB.Table("tracks").Count(&data.TrackCount)
	// lastfm box
	data.RequestRoot = c.BaseURL(r)
	data.CurrentLastFMAPIKey, _ = c.DB.GetSetting("lastfm_api_key")
	data.DefaultListenBrainzURL = listenbrainz.BaseURL

	// users box
	allUsersQ := c.DB.DB
	if !user.IsAdmin {
		allUsersQ = allUsersQ.Where("name=?", user.Name)
	}
	allUsersQ.Find(&data.AllUsers)

	// recent folders box
	c.DB.
		Where("tag_artist_id IS NOT NULL").
		Order("created_at DESC").
		Limit(8).
		Find(&data.RecentFolders)
	data.IsScanning = c.Scanner.IsScanning()
	if tStr, err := c.DB.GetSetting("last_scan_time"); err != nil {
		i, _ := strconv.ParseInt(tStr, 10, 64)
		data.LastScanTime = time.Unix(i, 0)
	}

	// transcoding box
	c.DB.
		Where("user_id=?", user.ID).
		Find(&data.TranscodePreferences)
	for profile := range transcode.UserProfiles {
		data.TranscodeProfiles = append(data.TranscodeProfiles, profile)
	}
	sort.Strings(data.TranscodeProfiles)
	// podcasts box
	c.DB.Find(&data.Podcasts)

	// internet radio box
	c.DB.Find(&data.InternetRadioStations)

	return &Response{
		template: "home.tmpl",
		data:     data,
	}
}

func (c *Controller) ServeLinkLastFMDo(r *http.Request) *Response {
	token := r.URL.Query().Get("token")
	if token == "" {
		return &Response{code: 400, err: "please provide a token"}
	}
	apiKey, err := c.DB.GetSetting("lastfm_api_key")
	if err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("couldn't get api key: %v", err)}}
	}
	secret, err := c.DB.GetSetting("lastfm_secret")
	if err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("couldn't get secret: %v", err)}}
	}
	sessionKey, err := lastfm.GetSession(apiKey, secret, token)
	if err != nil {
		return &Response{
			redirect: "/admin/home",
			flashW:   []string{err.Error()},
		}
	}
	user := r.Context().Value(CtxUser).(*db.User)
	user.LastFMSession = sessionKey
	if err := c.DB.Save(user).Error; err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("save user: %v", err)}}
	}
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeUnlinkLastFMDo(r *http.Request) *Response {
	user := r.Context().Value(CtxUser).(*db.User)
	user.LastFMSession = ""
	if err := c.DB.Save(user).Error; err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("save user: %v", err)}}
	}
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
	if err := c.DB.Save(user).Error; err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("save user: %v", err)}}
	}
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeUnlinkListenBrainzDo(r *http.Request) *Response {
	user := r.Context().Value(CtxUser).(*db.User)
	user.ListenBrainzURL = ""
	user.ListenBrainzToken = ""
	if err := c.DB.Save(user).Error; err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("save user: %v", err)}}
	}
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeChangeUsername(r *http.Request) *Response {
	user, err := selectedUserIfAdmin(c, r)
	if err != nil {
		return &Response{code: 400, err: err.Error()}
	}
	data := &templateData{}
	data.SelectedUser = user
	return &Response{
		template: "change_username.tmpl",
		data:     data,
	}
}

func (c *Controller) ServeChangeUsernameDo(r *http.Request) *Response {
	user, err := selectedUserIfAdmin(c, r)
	if err != nil {
		return &Response{code: 400, err: err.Error()}
	}
	usernameNew := r.FormValue("username")
	if err := validateUsername(usernameNew); err != nil {
		return &Response{
			redirect: r.Referer(),
			flashW:   []string{err.Error()},
		}
	}
	user.Name = usernameNew
	if err := c.DB.Save(user).Error; err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("save username: %v", err)}}
	}
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeChangePassword(r *http.Request) *Response {
	user, err := selectedUserIfAdmin(c, r)
	if err != nil {
		return &Response{code: 400, err: err.Error()}
	}
	data := &templateData{}
	data.SelectedUser = user
	return &Response{
		template: "change_password.tmpl",
		data:     data,
	}
}

func (c *Controller) ServeChangePasswordDo(r *http.Request) *Response {
	user, err := selectedUserIfAdmin(c, r)
	if err != nil {
		return &Response{code: 400, err: err.Error()}
	}
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	if err := validatePasswords(passwordOne, passwordTwo); err != nil {
		return &Response{
			redirect: r.Referer(),
			flashW:   []string{err.Error()},
		}
	}
	user.Password = passwordOne
	if err := c.DB.Save(user).Error; err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("save user: %v", err)}}
	}
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeChangeAvatar(r *http.Request) *Response {
	user, err := selectedUserIfAdmin(c, r)
	if err != nil {
		return &Response{code: 400, err: err.Error()}
	}
	data := &templateData{}
	data.SelectedUser = user
	return &Response{
		template: "change_avatar.tmpl",
		data:     data,
	}
}

func (c *Controller) ServeChangeAvatarDo(r *http.Request) *Response {
	user, err := selectedUserIfAdmin(c, r)
	if err != nil {
		return &Response{code: 400, err: err.Error()}
	}
	avatar, err := getAvatarFile(r)
	if err != nil {
		return &Response{
			redirect: r.Referer(),
			flashW:   []string{err.Error()},
		}
	}
	user.Avatar = avatar
	if err := c.DB.Save(user).Error; err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("save user: %v", err)}}
	}
	return &Response{
		redirect: r.Referer(),
		flashN:   []string{"avatar saved successfully"},
	}
}

func (c *Controller) ServeDeleteAvatarDo(r *http.Request) *Response {
	user, err := selectedUserIfAdmin(c, r)
	if err != nil {
		return &Response{code: 400, err: err.Error()}
	}
	user.Avatar = nil
	if err := c.DB.Save(user).Error; err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("save user: %v", err)}}
	}
	return &Response{
		redirect: r.Referer(),
		flashN:   []string{"avatar deleted successfully"},
	}
}

func (c *Controller) ServeDeleteUser(r *http.Request) *Response {
	user, err := selectedUserIfAdmin(c, r)
	if err != nil {
		return &Response{code: 400, err: err.Error()}
	}
	data := &templateData{}
	data.SelectedUser = user
	return &Response{
		template: "delete_user.tmpl",
		data:     data,
	}
}

func (c *Controller) ServeDeleteUserDo(r *http.Request) *Response {
	user, err := selectedUserIfAdmin(c, r)
	if err != nil {
		return &Response{code: 400, err: err.Error()}
	}
	if user.IsAdmin {
		return &Response{
			redirect: "/admin/home",
			flashW:   []string{"can't delete the admin user"},
		}
	}
	if err := c.DB.Delete(user).Error; err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("delete user: %v", err)}}
	}
	return &Response{redirect: "/admin/home"}
}

func (c *Controller) ServeCreateUser(r *http.Request) *Response {
	return &Response{template: "create_user.tmpl"}
}

func (c *Controller) ServeCreateUserDo(r *http.Request) *Response {
	username := r.FormValue("username")
	if err := validateUsername(username); err != nil {
		return &Response{
			redirect: r.Referer(),
			flashW:   []string{err.Error()},
		}
	}
	passwordOne := r.FormValue("password_one")
	passwordTwo := r.FormValue("password_two")
	if err := validatePasswords(passwordOne, passwordTwo); err != nil {
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
	var err error
	if data.CurrentLastFMAPIKey, err = c.DB.GetSetting("lastfm_api_key"); err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("couldn't get api key: %v", err)}}
	}
	if data.CurrentLastFMAPISecret, err = c.DB.GetSetting("lastfm_secret"); err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("couldn't get secret: %v", err)}}
	}
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
	if err := c.DB.SetSetting("lastfm_api_key", apiKey); err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("couldn't set api key: %v", err)}}
	}
	if err := c.DB.SetSetting("lastfm_secret", secret); err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("couldn't set secret: %v", err)}}
	}
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
		return &Response{code: 400, err: "please provide a client"}
	}
	c.DB.
		Where("user_id=? AND client=?", user.ID, client).
		Delete(db.TranscodePreference{})
	return &Response{
		redirect: "/admin/home",
	}
}

func (c *Controller) ServePodcastAddDo(r *http.Request) *Response {
	rssURL := r.FormValue("feed")
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(rssURL)
	if err != nil {
		return &Response{
			redirect: "/admin/home",
			flashW:   []string{fmt.Sprintf("could not create feed: %v", err)},
		}
	}
	if _, err := c.Podcasts.AddNewPodcast(rssURL, feed); err != nil {
		return &Response{
			redirect: "/admin/home",
			flashW:   []string{fmt.Sprintf("could not create feed: %v", err)},
		}
	}
	return &Response{
		redirect: "/admin/home",
	}
}

func (c *Controller) ServePodcastDownloadDo(r *http.Request) *Response {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		return &Response{code: 400, err: "please provide a valid podcast id"}
	}
	if err := c.Podcasts.DownloadPodcastAll(id); err != nil {
		return &Response{code: 400, err: "please provide a valid podcast id"}
	}
	return &Response{
		redirect: "/admin/home",
		flashN:   []string{"started downloading podcast episodes"},
	}
}

func (c *Controller) ServePodcastUpdateDo(r *http.Request) *Response {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		return &Response{code: 400, err: "please provide a valid podcast id"}
	}
	setting := db.PodcastAutoDownload(r.FormValue("setting"))
	var message string
	switch setting {
	case db.PodcastAutoDownloadLatest:
		message = "future podcast episodes will be automatically downloaded"
	case db.PodcastAutoDownloadNone:
		message = "future podcast episodes will not be downloaded"
	default:
		return &Response{code: 400, err: "please provide a valid podcast download type"}
	}
	if err := c.Podcasts.SetAutoDownload(id, setting); err != nil {
		return &Response{
			flashW: []string{fmt.Sprintf("could not update auto download setting: %v", err)},
			code:   400,
		}
	}
	return &Response{
		redirect: "/admin/home",
		flashN:   []string{message},
	}
}

func (c *Controller) ServePodcastDeleteDo(r *http.Request) *Response {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		return &Response{code: 400, err: "please provide a valid podcast id"}
	}
	if err := c.Podcasts.DeletePodcast(id); err != nil {
		return &Response{code: 400, err: "please provide a valid podcast id"}
	}
	return &Response{
		redirect: "/admin/home",
	}
}

func (c *Controller) ServeInternetRadioStationAddDo(r *http.Request) *Response {
	streamURL := r.FormValue("streamURL")
	name := r.FormValue("name")
	homepageURL := r.FormValue("homepageURL")

	if name == "" {
		return &Response{redirect: "/admin/home", flashW: []string{"no name provided"}}
	}

	if _, err := url.ParseRequestURI(streamURL); err != nil {
		return &Response{redirect: "/admin/home", flashW: []string{fmt.Sprintf("bad stream URL provided: %v", err)}}
	}

	if homepageURL != "" {
		if _, err := url.ParseRequestURI(homepageURL); err != nil {
			return &Response{redirect: "/admin/home", flashW: []string{fmt.Sprintf("bad homepage URL provided: %v", err)}}
		}
	}

	var station db.InternetRadioStation
	station.StreamURL = streamURL
	station.Name = name
	station.HomepageURL = homepageURL
	if err := c.DB.Save(&station).Error; err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("error saving station: %v", err)}}
	}

	return &Response{
		redirect: "/admin/home",
	}
}

func (c *Controller) ServeInternetRadioStationUpdateDo(r *http.Request) *Response {
	stationID, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		return &Response{code: 400, err: "please provide a valid internet radio station id"}
	}

	streamURL := r.FormValue("streamURL")
	name := r.FormValue("name")
	homepageURL := r.FormValue("homepageURL")

	if name == "" {
		return &Response{
			redirect: "/admin/home",
			flashW:   []string{"no name provided"},
		}
	}

	if _, err := url.ParseRequestURI(streamURL); err != nil {
		return &Response{
			redirect: "/admin/home",
			flashW:   []string{fmt.Sprintf("bad stream URL provided: %v", err)},
		}
	}

	if homepageURL != "" {
		if _, err := url.ParseRequestURI(homepageURL); err != nil {
			return &Response{
				redirect: "/admin/home",
				flashW:   []string{fmt.Sprintf("bad homepage URL provided: %v", err)},
			}
		}
	}

	var station db.InternetRadioStation
	if err := c.DB.Where("id=?", stationID).First(&station).Error; err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("find station by id: %v", err)}}
	}

	station.StreamURL = streamURL
	station.Name = name
	station.HomepageURL = homepageURL
	if err := c.DB.Save(&station).Error; err != nil {
		return &Response{code: 500, err: "please provide a valid internet radio station id"}
	}

	return &Response{
		redirect: "/admin/home",
	}
}

func (c *Controller) ServeInternetRadioStationDeleteDo(r *http.Request) *Response {
	stationID, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		return &Response{code: 400, err: "please provide a valid internet radio station id"}
	}

	var station db.InternetRadioStation
	if err := c.DB.Where("id=?", stationID).First(&station).Error; err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("find station by id: %v", err)}}
	}

	if err := c.DB.Where("id=?", stationID).Delete(&db.InternetRadioStation{}).Error; err != nil {
		return &Response{redirect: r.Referer(), flashW: []string{fmt.Sprintf("deleting radio station: %v", err)}}
	}

	return &Response{
		redirect: "/admin/home",
	}
}

func getAvatarFile(r *http.Request) ([]byte, error) {
	err := r.ParseMultipartForm(10 << 20) // keep up to 10MB in memory
	if err != nil {
		return nil, err
	}
	file, _, err := r.FormFile("avatar")
	if err != nil {
		return nil, fmt.Errorf("read form file: %w", err)
	}
	i, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	resized := resize.Resize(64, 64, i, resize.Lanczos3)
	var buff bytes.Buffer
	if err := jpeg.Encode(&buff, resized, nil); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

func selectedUserIfAdmin(c *Controller, r *http.Request) (*db.User, error) {
	selectedUsername := r.URL.Query().Get("user")
	if selectedUsername == "" {
		return nil, fmt.Errorf("please provide a username")
	}
	user := r.Context().Value(CtxUser).(*db.User)
	if !user.IsAdmin && user.Name != selectedUsername {
		return nil, fmt.Errorf("must be admin to perform actions for other users")
	}
	selectedUser := c.DB.GetUserByName(selectedUsername)
	return selectedUser, nil
}
