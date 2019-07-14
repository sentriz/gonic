package ctrlsubsonic

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"

	"senan.xyz/g/gonic/server/ctrlsubsonic/spec"
	"senan.xyz/g/gonic/server/key"
	"senan.xyz/g/gonic/server/parsing"
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

func checkCredsToken(password, token, salt string) bool {
	toHash := fmt.Sprintf("%s%s", password, salt)
	hash := md5.Sum([]byte(toHash))
	expToken := hex.EncodeToString(hash[:])
	return token == expToken
}

func checkCredsBasic(password, given string) bool {
	if len(given) >= 4 && given[:4] == "enc:" {
		bytes, _ := hex.DecodeString(given[4:])
		given = string(bytes)
	}
	return password == given
}

func (c *Controller) WithValidSubsonicArgs(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := checkHasAllParams(r.URL.Query()); err != nil {
			writeResp(w, r, spec.NewError(10, err.Error()))
			return
		}
		username := parsing.GetStrParam(r, "u")
		password := parsing.GetStrParam(r, "p")
		token := parsing.GetStrParam(r, "t")
		salt := parsing.GetStrParam(r, "s")
		passwordAuth := token == "" && salt == ""
		tokenAuth := password == ""
		if tokenAuth == passwordAuth {
			writeResp(w, r, spec.NewError(10, "please provide `t` and `s`, or just `p`"))
			return
		}
		user := c.DB.GetUserFromName(username)
		if user == nil {
			writeResp(w, r, spec.NewError(40, "invalid username `%s`", username))
			return
		}
		var credsOk bool
		if tokenAuth {
			credsOk = checkCredsToken(user.Password, token, salt)
		} else {
			credsOk = checkCredsBasic(user.Password, password)
		}
		if !credsOk {
			writeResp(w, r, spec.NewError(40, "invalid password"))
			return
		}
		withUser := context.WithValue(r.Context(), key.User, user)
		next.ServeHTTP(w, r.WithContext(withUser))
	})
}
