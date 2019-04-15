package handler

import (
	"net/http"

	"github.com/sentriz/gonic/db"
)

func (c *Controller) ServeLogin(w http.ResponseWriter, r *http.Request) {
	session, _ := c.SStore.Get(r, "gonic")
	renderTemplate(w, r, session, "login", &templateData{})
}

func (c *Controller) ServeAuthenticate(w http.ResponseWriter, r *http.Request) {
	session, _ := c.SStore.Get(r, "gonic")
	username := r.FormValue("username")
	password := r.FormValue("password")
	if username == "" || password == "" {
		session.AddFlash("please provide both a username and password")
		session.Save(r, w)
		http.Redirect(w, r, "/admin/login", 303)
		return
	}
	var user db.User
	c.DB.Where("name = ?", username).First(&user)
	if !(username == user.Name && password == user.Password) {
		session.AddFlash("invalid username / password")
		session.Save(r, w)
		http.Redirect(w, r, "/admin/login", 303)
		return
	}
	session.Values["authenticated"] = true
	session.Values["user"] = user.ID
	session.Save(r, w)
	http.Redirect(w, r, "/admin/home", 303)
}

func (c *Controller) ServeHome(w http.ResponseWriter, r *http.Request) {
	session, _ := c.SStore.Get(r, "gonic")
	authed, _ := session.Values["authenticated"].(bool)
	if !authed {
		session.AddFlash("you are not authenticated")
		session.Save(r, w)
		http.Redirect(w, r, "/admin/login", 303)
		return
	}
	var data templateData
	var user db.User
	c.DB.First(&user, session.Values["user"])
	data.UserID = user.ID
	data.Username = user.Name
	c.DB.Table("album_artists").Count(&data.ArtistCount)
	c.DB.Table("albums").Count(&data.AlbumCount)
	c.DB.Table("tracks").Count(&data.TrackCount)
	c.DB.Find(&data.Users)
	renderTemplate(w, r, session, "home", &data)
}

func (c *Controller) ServeCreateUser(w http.ResponseWriter, r *http.Request) {
	session, _ := c.SStore.Get(r, "gonic")
	authed, _ := session.Values["authenticated"].(bool)
	if !authed {
		session.AddFlash("you are not authenticated")
		session.Save(r, w)
		http.Redirect(w, r, "/admin/login", 303)
		return
	}
	renderTemplate(w, r, session, "create_user", &templateData{})
}

func (c *Controller) ServeLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := c.SStore.Get(r, "gonic")
	delete(session.Values, "authenticated")
	delete(session.Values, "user")
	session.Save(r, w)
	http.Redirect(w, r, "/admin/login", 303)
}
