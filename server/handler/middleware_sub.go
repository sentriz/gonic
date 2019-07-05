package handler

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
)

var requiredParameters = []string{
	"u", "v", "c",
}

func checkHasAllParams(params url.Values) error {
	for _, req := range requiredParameters {
		param := params.Get(req)
		if param != "" {
			continue
		}
		return fmt.Errorf("please provide a `%s` parameter", req)
	}
	return nil
}

func checkCredentialsToken(password, token, salt string) bool {
	toHash := fmt.Sprintf("%s%s", password, salt)
	hash := md5.Sum([]byte(toHash))
	expToken := hex.EncodeToString(hash[:])
	return token == expToken
}

func checkCredentialsBasic(password, given string) bool {
	if given[:4] == "enc:" {
		bytes, _ := hex.DecodeString(given[4:])
		given = string(bytes)
	}
	return password == given
}

//nolint:interfacer
func (c *Controller) WithValidSubsonicArgs(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := checkHasAllParams(r.URL.Query())
		if err != nil {
			respondError(w, r, 10, err.Error())
			return
		}
		username, password := r.URL.Query().Get("u"),
			r.URL.Query().Get("p")
		token, salt := r.URL.Query().Get("t"),
			r.URL.Query().Get("s")
		passwordAuth, tokenAuth := token == "" && salt == "",
			password == ""
		if tokenAuth == passwordAuth {
			respondError(w, r, 10,
				"please provide parameters `t` and `s`, or just `p`")
			return
		}
		user := c.DB.GetUserFromName(username)
		if user == nil {
			respondError(w, r, 40, "invalid username `%s`", username)
			return
		}
		var credsOk bool
		if tokenAuth {
			credsOk = checkCredentialsToken(user.Password, token, salt)
		} else {
			credsOk = checkCredentialsBasic(user.Password, password)
		}
		if !credsOk {
			respondError(w, r, 40, "invalid password")
			return
		}
		withUser := context.WithValue(r.Context(), contextUserKey, user)
		next.ServeHTTP(w, r.WithContext(withUser))
	}
}

//nolint:interfacer
func (c *Controller) WithCORS(next http.HandlerFunc) http.HandlerFunc {
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
