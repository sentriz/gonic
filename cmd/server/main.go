package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"

	"github.com/sentriz/gonic/context"
	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/handler"
	"github.com/sentriz/gonic/router"
	"github.com/sentriz/gonic/subsonic"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/labstack/echo"
)

var (
	username           = "senan"
	password           = "howdy"
	requiredParameters = []string{
		"u", "t", "s", "v", "c",
	}
)

func checkCredentials(token, salt string) bool {
	toHash := fmt.Sprintf("%s%s", password, salt)
	hash := md5.Sum([]byte(toHash))
	expToken := hex.EncodeToString(hash[:])
	return token == expToken
}

func contextMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return next(&context.Subsonic{c})
	}
}

func validationMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		cc := c.(*context.Subsonic)
		for _, req := range requiredParameters {
			param := cc.QueryParams().Get(req)
			if param != "" {
				continue
			}
			return cc.Respond(http.StatusBadRequest, subsonic.NewError(
				10, fmt.Sprintf("please provide a `%s` parameter", req),
			))
		}
		credsOk := checkCredentials(
			cc.QueryParams().Get("t"), // token
			cc.QueryParams().Get("s"), // salt
		)
		if !credsOk {
			return cc.Respond(http.StatusBadRequest, subsonic.NewError(
				40, "invalid username or password",
			))
		}
		return next(c)
	}
}

func main() {
	d := db.New()
	r := router.New()
	r.Use(contextMiddleware)
	r.Use(validationMiddleware)
	h := &handler.Handler{
		DB:     d,
		Router: r,
	}
	rest := r.Group("/rest")
	rest.GET("", h.GetTest)
	log.Fatal(r.Start("127.0.0.1:5001"))
}
