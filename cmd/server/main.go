package main

import (
	"log"

	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/handler"
	"github.com/sentriz/gonic/router"
)

func main() {
	d := db.New()
	r := router.New()
	h := &handler.Handler{
		DB:     d,
		Router: r,
	}
	rest := r.Group("/rest")
	rest.GET("", h.GetTest)
	log.Fatal(r.Start("127.0.0.1:5001"))
}
