package ctrladmin

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"

	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/version"
)

func (c *Controller) WithSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := c.sessDB.Get(r, version.NAME)
		if err != nil {
			http.Error(w, fmt.Sprintf("error getting session: %s", err), 500)
			return
		}
		withSession := context.WithValue(r.Context(), CtxSession, session)
		next.ServeHTTP(w, r.WithContext(withSession))
	})
}

func (c *Controller) WithUserSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// session exists at this point
		session := r.Context().Value(CtxSession).(*sessions.Session)
		userID, ok := session.Values["user"].(int)
		if !ok {
			sessAddFlashW(session, []string{"you are not authenticated"})
			sessLogSave(session, w, r)
			http.Redirect(w, r, c.Path("/admin/login"), http.StatusSeeOther)
			return
		}
		// take username from sesion and add the user row to the context
		user := c.DB.GetUserByID(userID)
		if user == nil {
			// the username in the client's session no longer relates to a
			// user in the database (maybe the user was deleted)
			session.Options.MaxAge = -1
			sessLogSave(session, w, r)
			http.Redirect(w, r, c.Path("/admin/login"), http.StatusSeeOther)
			return
		}
		withUser := context.WithValue(r.Context(), CtxUser, user)
		next.ServeHTTP(w, r.WithContext(withUser))
	})
}

func (c *Controller) WithAdminSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// session and user exist at this point
		session := r.Context().Value(CtxSession).(*sessions.Session)
		user := r.Context().Value(CtxUser).(*db.User)
		if !user.IsAdmin {
			sessAddFlashW(session, []string{"you are not an admin"})
			sessLogSave(session, w, r)
			http.Redirect(w, r, c.Path("/admin/login"), http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
