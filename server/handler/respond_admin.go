package handler

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/sessions"

	"github.com/sentriz/gonic/model"
)

type templateData struct {
	// common
	Flashes []interface{}
	User    *model.User
	// home
	AlbumCount    int
	ArtistCount   int
	TrackCount    int
	RequestRoot   string
	RecentFolders []*model.Album
	AllUsers      []*model.User
	LastScanTime  time.Time
	//
	CurrentLastFMAPIKey    string
	CurrentLastFMAPISecret string
	SelectedUser           *model.User
}

func renderTemplate(
	w http.ResponseWriter,
	r *http.Request,
	tmpl *template.Template,
	data *templateData,
) {
	if data == nil {
		data = &templateData{}
	}
	session := r.Context().Value(contextSessionKey).(*sessions.Session)
	data.Flashes = session.Flashes()
	sessionLogSave(w, r, session)
	data.User, _ = r.Context().Value(contextUserKey).(*model.User)
	err := tmpl.Execute(w, data)
	if err != nil {
		log.Printf("error executing template: %v\n", err)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
}
