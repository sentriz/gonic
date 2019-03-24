package handler

import (
	"github.com/jinzhu/gorm"
	"github.com/labstack/echo"
)

// Handler is passed to the handler functions so
// they can access the database
type Handler struct {
	DB     *gorm.DB
	Router *echo.Echo
}
