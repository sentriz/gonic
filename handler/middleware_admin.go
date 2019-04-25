package handler

import (
	"context"
	"net/http"

	"github.com/gorilla/sessions"

	"github.com/sentriz/gonic/db"
)

func (c *Controller) WithSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := c.SStore.Get(r, "gonic")
		withSession := context.WithValue(r.Context(), contextSessionKey, session)
		next.ServeHTTP(w, r.WithContext(withSession))
	}
}

func (c *Controller) WithUserSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// session exists at this point
		session := r.Context().Value(contextSessionKey).(*sessions.Session)
		username, ok := session.Values["user"].(string)
		if !ok {
			session.AddFlash("you are not authenticated")
			session.Save(r, w)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		// take username from sesion and add the user row to the context
		user := c.GetUserFromName(username)
		if user.ID == 0 {
			// the username in the client's session no longer relates to a
			// user in the database (maybe the user was deleted)
			session.Options.MaxAge = -1
			session.Save(r, w)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		withUser := context.WithValue(r.Context(), contextUserKey, user)
		next.ServeHTTP(w, r.WithContext(withUser))
	}
}

func (c *Controller) WithAdminSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// session and user exist at this point
		session := r.Context().Value(contextSessionKey).(*sessions.Session)
		user := r.Context().Value(contextUserKey).(*db.User)
		if !user.IsAdmin {
			session.AddFlash("you are not an admin")
			session.Save(r, w)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	}
}
