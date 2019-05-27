package handler

import (
	"html/template"

	"github.com/jinzhu/gorm"
	"github.com/wader/gormstore"

	"github.com/sentriz/gonic/model"
)

type contextKey int

const (
	contextUserKey contextKey = iota
	contextSessionKey
)

type Controller struct {
	DB        *gorm.DB
	SessDB    *gormstore.Store
	Templates map[string]*template.Template
	MusicPath string
}

func (c *Controller) GetSetting(key string) string {
	var setting model.Setting
	c.DB.Where("key = ?", key).First(&setting)
	return setting.Value
}

func (c *Controller) SetSetting(key, value string) {
	c.DB.
		Where(model.Setting{Key: key}).
		Assign(model.Setting{Value: value}).
		FirstOrCreate(&model.Setting{})
}

func (c *Controller) GetUserFromName(name string) *model.User {
	var user model.User
	err := c.DB.
		Where("name = ?", name).
		First(&user).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return nil
	}
	return &user
}
