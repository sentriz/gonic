package handler

import (
	"html/template"

	"github.com/jinzhu/gorm"
	"github.com/wader/gormstore"

	"github.com/sentriz/gonic/db"
)

type Controller struct {
	DB        *gorm.DB                      // common
	SStore    *gormstore.Store              // admin
	Templates map[string]*template.Template // admin
}

func (c *Controller) GetSetting(key string) string {
	var setting db.Setting
	c.DB.Where("key = ?", key).First(&setting)
	return setting.Value
}

func (c *Controller) SetSetting(key, value string) {
	c.DB.
		Where(db.Setting{Key: key}).
		Assign(db.Setting{Value: value}).
		FirstOrCreate(&db.Setting{})
}

func (c *Controller) GetUserFromName(name string) *db.User {
	var user db.User
	c.DB.Where("name = ?", name).First(&user)
	return &user
}
