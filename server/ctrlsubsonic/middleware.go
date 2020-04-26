package ctrlsubsonic

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"

	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
)

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

func (c *Controller) WithParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := params.New(r)
		withParams := context.WithValue(r.Context(), CtxParams, params)
		next.ServeHTTP(w, r.WithContext(withParams))
	})
}

func (c *Controller) WithRequiredParams(next http.Handler) http.Handler {
	requiredParameters := []string{
		"u", "v", "c",
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := r.Context().Value(CtxParams).(params.Params)
		for _, req := range requiredParameters {
			if _, err := params.Get(req); err != nil {
				_ = writeResp(w, r, spec.NewError(10,
					"please provide a `%s` parameter", req))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (c *Controller) WithUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := r.Context().Value(CtxParams).(params.Params)
		// ignoring errors here, a middleware has already ensured they exist
		username, _ := params.Get("u")
		password, _ := params.Get("p")
		token, _ := params.Get("t")
		salt, _ := params.Get("s")
		//
		passwordAuth := token == "" && salt == ""
		tokenAuth := password == ""
		if tokenAuth == passwordAuth {
			_ = writeResp(w, r, spec.NewError(10,
				"please provide `t` and `s`, or just `p`"))
			return
		}
		user := c.DB.GetUserFromName(username)
		if user == nil {
			_ = writeResp(w, r, spec.NewError(40,
				"invalid username `%s`", username))
			return
		}
		var credsOk bool
		if tokenAuth {
			credsOk = checkCredsToken(user.Password, token, salt)
		} else {
			credsOk = checkCredsBasic(user.Password, password)
		}
		if !credsOk {
			_ = writeResp(w, r, spec.NewError(40, "invalid password"))
			return
		}
		withUser := context.WithValue(r.Context(), CtxUser, user)
		next.ServeHTTP(w, r.WithContext(withUser))
	})
}
