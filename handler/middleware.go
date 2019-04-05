package handler

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"

	"github.com/jinzhu/gorm"
	"github.com/sentriz/gonic/db"
)

var requiredParameters = []string{
	"u", "v", "c",
}

func checkCredentialsNewWay(password, token, salt string) bool {
	toHash := fmt.Sprintf("%s%s", password, salt)
	hash := md5.Sum([]byte(toHash))
	expToken := hex.EncodeToString(hash[:])
	return token == expToken
}

func checkCredentialsOldWay(password, givenPassword string) bool {
	if givenPassword[:4] == "enc:" {
		bytes, _ := hex.DecodeString(givenPassword[4:])
		givenPassword = string(bytes)
	}
	return password == givenPassword
}

func (c *Controller) LogConnection(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("connection from `%s` for `%s`", r.RemoteAddr, r.URL)
		next.ServeHTTP(w, r)
	}
}

func (c *Controller) CheckParameters(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, req := range requiredParameters {
			param := r.URL.Query().Get(req)
			if param != "" {
				continue
			}
			respondError(w, r,
				10, fmt.Sprintf("please provide a `%s` parameter", req),
			)
			return
		}
		username := r.URL.Query().Get("u")
		password := r.URL.Query().Get("p")
		token := r.URL.Query().Get("t")
		salt := r.URL.Query().Get("s")
		passwordAuth := token == "" && salt == ""
		tokenAuth := password == ""
		if tokenAuth == passwordAuth {
			respondError(w, r,
				10, "please provide parameters `t` and `s`, or just `p`",
			)
			return
		}
		user := db.User{
			Username: username,
		}
		err := c.DB.Where(user).First(&user).Error
		if gorm.IsRecordNotFoundError(err) {
			respondError(w, r, 40, "invalid username")
			return
		}
		var credsOk bool
		if tokenAuth {
			credsOk = checkCredentialsNewWay(user.Password, token, salt)
		} else {
			credsOk = checkCredentialsOldWay(user.Password, password)
		}
		if !credsOk {
			respondError(w, r, 40, "invalid password")
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (c *Controller) EnableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods",
			"POST, GET, OPTIONS, PUT, DELETE",
		)
		w.Header().Set("Access-Control-Allow-Headers",
			"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization",
		)
		if r.Method == "OPTIONS" {
			return
		}
		next.ServeHTTP(w, r)
	}
}
