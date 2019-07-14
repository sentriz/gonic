package ctrladmin

import (
	"context"
	"net/http"

	"github.com/gorilla/sessions"

	"senan.xyz/g/gonic/model"
	"senan.xyz/g/gonic/server/key"
)

func (c *Controller) WithSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := c.sessDB.Get(r, "gonic")
		withSession := context.WithValue(r.Context(), key.Session, session)
		next.ServeHTTP(w, r.WithContext(withSession))
	})
}

func (c *Controller) WithUserSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// session exists at this point
		session := r.Context().Value(key.Session).(*sessions.Session)
		username, ok := session.Values["user"].(string)
		if !ok {
			sessAddFlashW("you are not authenticated", session)
			sessLogSave(w, r, session)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		// take username from sesion and add the user row to the context
		user := c.DB.GetUserFromName(username)
		if user == nil {
			// the username in the client's session no longer relates to a
			// user in the database (maybe the user was deleted)
			session.Options.MaxAge = -1
			sessLogSave(w, r, session)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		withUser := context.WithValue(r.Context(), key.User, user)
		next.ServeHTTP(w, r.WithContext(withUser))
	})
}

func (c *Controller) WithAdminSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// session and user exist at this point
		session := r.Context().Value(key.Session).(*sessions.Session)
		user := r.Context().Value(key.User).(*model.User)
		if !user.IsAdmin {
			sessAddFlashW("you are not an admin", session)
			sessLogSave(w, r, session)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
