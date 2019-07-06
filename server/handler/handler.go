package handler

import (
	"html/template"

	"github.com/wader/gormstore"

	"senan.xyz/g/gonic/db"
)

type contextKey int

const (
	contextUserKey contextKey = iota
	contextSessionKey
)

type Controller struct {
	DB        *db.DB
	SessDB    *gormstore.Store
	Templates map[string]*template.Template
	MusicPath string
}
