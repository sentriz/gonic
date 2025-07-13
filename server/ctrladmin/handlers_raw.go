package ctrladmin

import (
	"net/http"

	"github.com/gorilla/sessions"
)

func (c *Controller) ServeLoginDo(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(CtxSession).(*sessions.Session)

	// Check if password authentication is allowed
	if GetAuthMethod() != "password" {
		sessAddFlashW(session, []string{"password authentication is not available"})
		sessLogSave(session, w, r)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	if username == "" || password == "" {
		sessAddFlashW(session, []string{"please provide username and password"})
		sessLogSave(session, w, r)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}
	user := c.dbc.GetUserByName(username)
	if user == nil || password != user.Password {
		sessAddFlashW(session, []string{"invalid username / password"})
		sessLogSave(session, w, r)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}
	// put the user name into the session. future endpoints after this one
	// are wrapped with WithUserSession() which will get the name from the
	// session and put the row into the request context
	session.Values["user"] = user.ID
	sessLogSave(session, w, r)
	http.Redirect(w, r, c.resolveProxyPath("/admin/home"), http.StatusSeeOther)
}

func (c *Controller) ServeLogout(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(CtxSession).(*sessions.Session)
	session.Options.MaxAge = -1
	sessLogSave(session, w, r)
	http.Redirect(w, r, c.resolveProxyPath("/admin/login"), http.StatusSeeOther)
}
