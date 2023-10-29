package ctrlbase

import (
	"fmt"
	"net/http"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func AddRoutes(c *Controller, r *mux.Router, logHTTP bool) {
	if logHTTP {
		r.Use(c.WithLogging)
	}
	r.Use(c.WithCORS)
	r.Use(handlers.RecoveryHandler(handlers.PrintRecoveryStack(true)))

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		adminHome := c.Path("/admin/home")
		http.Redirect(w, r, adminHome, http.StatusSeeOther)
	})
	// misc subsonic routes without /rest prefix
	r.HandleFunc("/settings.view", func(w http.ResponseWriter, r *http.Request) {
		adminHome := c.Path("/admin/home")
		http.Redirect(w, r, adminHome, http.StatusSeeOther)
	})
	r.HandleFunc("/musicFolderSettings.view", func(w http.ResponseWriter, r *http.Request) {
		restScan := c.Path(fmt.Sprintf("/rest/startScan.view?%s", r.URL.Query().Encode()))
		http.Redirect(w, r, restScan, http.StatusSeeOther)
	})
	r.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "OK")
	})
}
