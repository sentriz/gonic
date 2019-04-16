package handler

import (
	"fmt"
	"net/http"

	"github.com/jinzhu/gorm"
	"github.com/sentriz/gonic/db"
)

func (c *Controller) ServeLogin(w http.ResponseWriter, r *http.Request) {
	session, _ := c.SStore.Get(r, "gonic")
	renderTemplate(w, r, session, "login", &templateData{})
}

func (c *Controller) ServeLoginDo(w http.ResponseWriter, r *http.Request) {
	session, _ := c.SStore.Get(r, "gonic")
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
	session, _ := c.SStore.Get(r, "gonic")
	delete(session.Values, "user")
	session.Save(r, w)
	http.Redirect(w, r, "/admin/login", 303)
}

func (c *Controller) ServeHome(w http.ResponseWriter, r *http.Request) {
	session, _ := c.SStore.Get(r, "gonic")
	var data templateData
	c.DB.Table("album_artists").Count(&data.ArtistCount)
	c.DB.Table("albums").Count(&data.AlbumCount)
	c.DB.Table("tracks").Count(&data.TrackCount)
	c.DB.Find(&data.AllUsers)
	renderTemplate(w, r, session, "home", &data)
}

func (c *Controller) ServeChangePassword(w http.ResponseWriter, r *http.Request) {
	session, _ := c.SStore.Get(r, "gonic")
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
	renderTemplate(w, r, session, "change_password", &data)
}

func (c *Controller) ServeChangePasswordDo(w http.ResponseWriter, r *http.Request) {
	session, _ := c.SStore.Get(r, "gonic")
	username := r.URL.Query().Get("user")
	var user db.User
	c.DB.Where("name = ?", username).First(&user)
	password_one := r.FormValue("password_one")
	password_two := r.FormValue("password_two")
	if password_one == "" || password_two == "" {
		session.AddFlash("please provide both passwords")
		session.Save(r, w)
		http.Redirect(w, r, r.Header.Get("Referer"), 302)
		return
	}
	if !(password_one == password_two) {
		session.AddFlash("the two passwords entered were not the same")
		session.Save(r, w)
		http.Redirect(w, r, r.Header.Get("Referer"), 302)
		return
	}
	user.Password = password_one
	c.DB.Save(&user)
	http.Redirect(w, r, "/admin/home", 303)
}

func (c *Controller) ServeCreateUser(w http.ResponseWriter, r *http.Request) {
	session, _ := c.SStore.Get(r, "gonic")
	renderTemplate(w, r, session, "create_user", &templateData{})
}

func (c *Controller) ServeCreateUserDo(w http.ResponseWriter, r *http.Request) {
	session, _ := c.SStore.Get(r, "gonic")
	username := r.FormValue("username")
	password_one := r.FormValue("password_one")
	password_two := r.FormValue("password_two")
	if username == "" || password_one == "" || password_two == "" {
		session.AddFlash("please fill out all fields")
		session.Save(r, w)
		http.Redirect(w, r, r.Header.Get("Referer"), 302)
		return
	}
	if !(password_one == password_two) {
		session.AddFlash("the two passwords entered were not the same")
		session.Save(r, w)
		http.Redirect(w, r, r.Header.Get("Referer"), 302)
		return
	}
	user := db.User{
		Name:     username,
		Password: password_one,
	}
	err := c.DB.Create(&user).Error
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
