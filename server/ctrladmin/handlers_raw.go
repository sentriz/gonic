package ctrladmin

import (
	"net/http"

	"github.com/gorilla/sessions"
	"go.senan.xyz/gonic/ldap"
)

func (c *Controller) ServeLoginDo(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(CtxSession).(*sessions.Session)
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
		// Because internal authentication failed, we can now try to use LDAP, if
		// it was enabled by the user.
		ok, err := ldap.CheckLDAPcreds(username, password, c.dbc)
		if err != nil {
			sessAddFlashW(session, []string{err.Error()})
		} else if !ok {
			sessAddFlashW(session, []string{"invalid username / password"})

			sessLogSave(session, w, r)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}
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
