package router

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

// New creates a new Echo instance
func New() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
		},
		AllowMethods: []string{
			echo.GET,
			echo.HEAD,
			echo.PUT,
			echo.PATCH,
			echo.POST,
			echo.DELETE,
		},
	}))
	return e
}
