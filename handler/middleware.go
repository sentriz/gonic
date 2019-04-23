package handler

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"

	"github.com/sentriz/gonic/db"
)

var requiredParameters = []string{
	"u", "v", "c",
}

func checkCredentialsToken(password, token, salt string) bool {
	toHash := fmt.Sprintf("%s%s", password, salt)
	hash := md5.Sum([]byte(toHash))
	expToken := hex.EncodeToString(hash[:])
	return token == expToken
}

func checkCredentialsBasic(password, givenPassword string) bool {
	if givenPassword[:4] == "enc:" {
		bytes, _ := hex.DecodeString(givenPassword[4:])
		givenPassword = string(bytes)
	}
	return password == givenPassword
}

func (c *Controller) WithLogging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("connection from `%s` for `%s`", r.RemoteAddr, r.URL)
		next.ServeHTTP(w, r)
	}
}

func (c *Controller) WithValidSubsonicArgs(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, req := range requiredParameters {
			param := r.URL.Query().Get(req)
			if param != "" {
				continue
			}
			respondError(w, r,
				10, fmt.Sprintf("please provide a `%s` parameter", req),
			)
			return
		}
		username := r.URL.Query().Get("u")
		password := r.URL.Query().Get("p")
		token := r.URL.Query().Get("t")
		salt := r.URL.Query().Get("s")
		passwordAuth := token == "" && salt == ""
		tokenAuth := password == ""
		if tokenAuth == passwordAuth {
			respondError(w, r,
				10, "please provide parameters `t` and `s`, or just `p`",
			)
			return
		}
		user := db.User{
			Name: username,
		}
		err := c.DB.Where(user).First(&user).Error
		if gorm.IsRecordNotFoundError(err) {
			respondError(w, r, 40, "invalid username")
			return
		}
		var credsOk bool
		if tokenAuth {
			credsOk = checkCredentialsToken(user.Password, token, salt)
		} else {
			credsOk = checkCredentialsBasic(user.Password, password)
		}
		if !credsOk {
			respondError(w, r, 40, "invalid password")
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (c *Controller) WithCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods",
			"POST, GET, OPTIONS, PUT, DELETE",
		)
		w.Header().Set("Access-Control-Allow-Headers",
			"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization",
		)
		if r.Method == "OPTIONS" {
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (c *Controller) WithSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := c.SStore.Get(r, "gonic")
		withSession := context.WithValue(r.Context(), "session", session)
		next.ServeHTTP(w, r.WithContext(withSession))
	}
}

func (c *Controller) WithUserSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// session exists at this point
		session := r.Context().Value("session").(*sessions.Session)
		username, ok := session.Values["user"].(string)
		if !ok {
			session.AddFlash("you are not authenticated")
			session.Save(r, w)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		// take username from sesion and add the user row
		user := c.GetUserFromName(username)
		withUser := context.WithValue(r.Context(), "user", user)
		next.ServeHTTP(w, r.WithContext(withUser))
	}
}

func (c *Controller) WithAdminSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// session and user exist at this point
		session := r.Context().Value("session").(*sessions.Session)
		user := r.Context().Value("user").(*db.User)
		if !user.IsAdmin {
			session.AddFlash("you are not an admin")
			session.Save(r, w)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	}
}
