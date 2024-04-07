package ctrladmin

import (
	"log"
	"net/http"

	"github.com/gorilla/sessions"
	"go.senan.xyz/gonic/ldap"
)

func (c *Controller) ServeLoginDo(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(CtxSession).(*sessions.Session)
	username := r.FormValue("username")
	password := r.FormValue("password")
	user := c.dbc.GetUserByName(username)
	if username == "" || password == "" {
		sessAddFlashW(session, []string{"please provide username and password"})
		sessLogSave(session, w, r)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	if c.ldapConfig.IsSetup() {
		ok, err := ldap.CheckLDAPcreds(username, password, c.dbc, c.ldapConfig)
		if err != nil {
			log.Println("Failed to check LDAP credentials:", err)
			sessAddFlashW(session, []string{"failed to check LDAP credentials"})
		} else if !ok {
			sessAddFlashW(session, []string{"invalid username / password"})
			sessLogSave(session, w, r)
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}
	} else if user == nil {
		sessAddFlashW(session, []string{"invalid username / password"})
		sessLogSave(session, w, r)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	} else if user.Password != password {
		sessAddFlashW(session, []string{"invalid username / password"})
		sessLogSave(session, w, r)
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	user = c.dbc.GetUserByName(username)

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
