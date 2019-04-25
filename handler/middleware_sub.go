package handler

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
)

var (
	requiredParameters = []string{
		"u", "v", "c",
	}
)

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

func checkCredentialsBasic(password, givenPassword string) bool {
	if givenPassword[:4] == "enc:" {
		bytes, _ := hex.DecodeString(givenPassword[4:])
		givenPassword = string(bytes)
	}
	return password == givenPassword
}

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
			respondError(w, r,
				10, "please provide parameters `t` and `s`, or just `p`",
			)
			return
		}
		user := c.GetUserFromName(username)
		if user.ID == 0 {
			// the user does not exist
			respondError(w, r, 40, "invalid username")
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
		next.ServeHTTP(w, r)
	}
}

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
