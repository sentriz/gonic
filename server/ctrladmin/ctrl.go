package ctrladmin

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/dustin/go-humanize"
	"github.com/fatih/structs"
	"github.com/gorilla/sessions"
	"github.com/philippta/go-template/html/template"
	"github.com/sentriz/gormstore"

	"go.senan.xyz/gonic"
	"go.senan.xyz/gonic/auth"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/handlerutil"
	"go.senan.xyz/gonic/lastfm"
	"go.senan.xyz/gonic/podcast"
	"go.senan.xyz/gonic/scanner"
	"go.senan.xyz/gonic/server/ctrladmin/adminui"
)

type CtxKey int

const (
	CtxUser CtxKey = iota
	CtxSession
)

const (
	AuthMethodPassword    = "password"
	AuthMethodOIDC        = "oidc"
	AuthMethodOIDCForward = "oidc-forward"
)

// Global auth method configuration
var authMethod string

type Controller struct {
	*http.ServeMux

	dbc              *db.DB
	sessDB           *gormstore.Store
	scanner          *scanner.Scanner
	podcasts         *podcast.Podcasts
	lastfmClient     *lastfm.Client
	resolveProxyPath ProxyPathResolver
}

type ProxyPathResolver func(in string) string

func New(dbc *db.DB, sessDB *gormstore.Store, scanner *scanner.Scanner, podcasts *podcast.Podcasts, lastfmClient *lastfm.Client, resolveProxyPath ProxyPathResolver) (*Controller, error) {
	c := Controller{
		ServeMux: http.NewServeMux(),

		dbc:              dbc,
		sessDB:           sessDB,
		scanner:          scanner,
		podcasts:         podcasts,
		lastfmClient:     lastfmClient,
		resolveProxyPath: resolveProxyPath,
	}

	resp := respHandler(adminui.TemplatesFS, resolveProxyPath)

	baseChain := withSession(sessDB)
	userChain := handlerutil.Chain(
		baseChain,
		withUserSession(dbc, resolveProxyPath),
	)
	adminChain := handlerutil.Chain(
		userChain,
		withAdminSession,
	)

	c.Handle("/static/", http.FileServer(http.FS(adminui.StaticFS)))

	// public routes (creates session)
	c.Handle("/login", baseChain(resp(c.ServeLogin)))
	c.Handle("/login_do", baseChain(respRaw(c.ServeLoginDo)))
	c.Handle("/oidc/callback", baseChain(respRaw(func(w http.ResponseWriter, r *http.Request) {
		serveOIDCCallback(dbc, w, r, resolveProxyPath)
	})))
	c.Handle("/auth-error", baseChain(resp(c.ServeAuthError)))

	// user routes (if session is valid)
	c.Handle("/logout", userChain(respRaw(c.ServeLogout)))
	c.Handle("/home", userChain(resp(c.ServeHome)))
	c.Handle("/change_username", userChain(resp(c.ServeChangeUsername)))
	c.Handle("/change_username_do", userChain(resp(c.ServeChangeUsernameDo)))
	c.Handle("/change_password", userChain(resp(c.ServeChangePassword)))
	c.Handle("/change_password_do", userChain(resp(c.ServeChangePasswordDo)))
	c.Handle("/reveal_password", userChain(resp(c.ServeRevealPassword)))
	c.Handle("/change_avatar", userChain(resp(c.ServeChangeAvatar)))
	c.Handle("/change_avatar_do", userChain(resp(c.ServeChangeAvatarDo)))
	c.Handle("/delete_avatar_do", userChain(resp(c.ServeDeleteAvatarDo)))
	c.Handle("/delete_user", userChain(resp(c.ServeDeleteUser)))
	c.Handle("/delete_user_do", userChain(resp(c.ServeDeleteUserDo)))
	c.Handle("/link_lastfm_do", userChain(resp(c.ServeLinkLastFMDo)))
	c.Handle("/unlink_lastfm_do", userChain(resp(c.ServeUnlinkLastFMDo)))
	c.Handle("/link_listenbrainz_do", userChain(resp(c.ServeLinkListenBrainzDo)))
	c.Handle("/unlink_listenbrainz_do", userChain(resp(c.ServeUnlinkListenBrainzDo)))
	c.Handle("/create_transcode_pref_do", userChain(resp(c.ServeCreateTranscodePrefDo)))
	c.Handle("/delete_transcode_pref_do", userChain(resp(c.ServeDeleteTranscodePrefDo)))

	// admin routes (if session is valid, and is admin)
	c.Handle("/create_user", adminChain(resp(c.ServeCreateUser)))
	c.Handle("/create_user_do", adminChain(resp(c.ServeCreateUserDo)))
	c.Handle("/update_lastfm_api_key", adminChain(resp(c.ServeUpdateLastFMAPIKey)))
	c.Handle("/update_lastfm_api_key_do", adminChain(resp(c.ServeUpdateLastFMAPIKeyDo)))
	c.Handle("/start_scan_inc_do", adminChain(resp(c.ServeStartScanIncDo)))
	c.Handle("/start_scan_full_do", adminChain(resp(c.ServeStartScanFullDo)))
	c.Handle("/add_podcast_do", adminChain(resp(c.ServePodcastAddDo)))
	c.Handle("/delete_podcast_do", adminChain(resp(c.ServePodcastDeleteDo)))
	c.Handle("/download_podcast_do", adminChain(resp(c.ServePodcastDownloadDo)))
	c.Handle("/update_podcast_do", adminChain(resp(c.ServePodcastUpdateDo)))
	c.Handle("/add_internet_radio_station_do", adminChain(resp(c.ServeInternetRadioStationAddDo)))
	c.Handle("/delete_internet_radio_station_do", adminChain(resp(c.ServeInternetRadioStationDeleteDo)))
	c.Handle("/update_internet_radio_station_do", adminChain(resp(c.ServeInternetRadioStationUpdateDo)))

	c.Handle("/", baseChain(resp(c.ServeNotFound)))

	return &c, nil
}

func withSession(sessDB *gormstore.Store) handlerutil.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := sessDB.Get(r, gonic.Name)
			if err != nil {
				http.Error(w, fmt.Sprintf("error getting session: %s", err), 500)
				return
			}
			withSession := context.WithValue(r.Context(), CtxSession, session)
			next.ServeHTTP(w, r.WithContext(withSession))
		})
	}
}

func SetAuthMethod(method string) {
	authMethod = method
}

func GetAuthMethod() string {
	return authMethod
}

func withUserSession(dbc *db.DB, resolvePath func(string) string) handlerutil.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handle different auth methods
			switch GetAuthMethod() {
			case AuthMethodPassword:
				// Password authentication - check session only
				session := r.Context().Value(CtxSession).(*sessions.Session)
				userID, ok := session.Values["user"].(int)
				if !ok {
					sessAddFlashW(session, []string{"you are not authenticated"})
					sessLogSave(session, w, r)
					http.Redirect(w, r, resolvePath("/admin/login"), http.StatusSeeOther)
					return
				}
				user := dbc.GetUserByID(userID)
				if user == nil {
					session.Options.MaxAge = -1
					sessLogSave(session, w, r)
					http.Redirect(w, r, "/", http.StatusSeeOther)
					return
				}
				withUser := context.WithValue(r.Context(), CtxUser, user)
				next.ServeHTTP(w, r.WithContext(withUser))
				return

			case AuthMethodOIDC:
				// OIDC authentication - check session first, then redirect to authorization
				session := r.Context().Value(CtxSession).(*sessions.Session)
				userID, ok := session.Values["user"].(int)
				if !ok {
					oidcURL := auth.BuildOIDCAuthURL(auth.GetOIDCAuthEndpoint(), r)
					log.Printf("No session found, redirecting to OIDC authorization: %s", oidcURL)
					http.Redirect(w, r, oidcURL, http.StatusSeeOther)
					return
				}
				user := dbc.GetUserByID(userID)
				if user == nil {
					session.Options.MaxAge = -1
					sessLogSave(session, w, r)
					oidcURL := auth.BuildOIDCAuthURL(auth.GetOIDCAuthEndpoint(), r)
					http.Redirect(w, r, oidcURL, http.StatusSeeOther)
					return
				}
				withUser := context.WithValue(r.Context(), CtxUser, user)
				next.ServeHTTP(w, r.WithContext(withUser))
				return

			case AuthMethodOIDCForward:
				// OIDC-forward authentication - JWT required in configured header
				authHeader := r.Header.Get(auth.GetOIDCHeader())
				if authHeader == "" {
					log.Printf("No %s header found for oidc-forward authentication", auth.GetOIDCHeader())
					http.Redirect(w, r, resolvePath("/admin/auth-error"), http.StatusSeeOther)
					return
				}

				jwtToken := authHeader
				if strings.HasPrefix(authHeader, "Bearer ") {
					jwtToken = authHeader[7:]
				}

				claims, err := auth.ValidateIncomingJWT(jwtToken)
				if err != nil {
					log.Printf("JWT validation failed: %v", err)
					http.Redirect(w, r, resolvePath("/admin/auth-error"), http.StatusSeeOther)
					return
				}

				log.Printf("JWT validation successful for user: %s", claims.Subject)

				session := r.Context().Value(CtxSession).(*sessions.Session)
				user, err := auth.HandleOIDCLogin(dbc, session, claims)
				if err != nil {
					log.Printf("Error handling OIDC login from JWT: %v", err)
					http.Error(w, "Failed to handle OIDC login", 500)
					return
				}
				sessLogSave(session, w, r)

				withUser := context.WithValue(r.Context(), CtxUser, user)
				next.ServeHTTP(w, r.WithContext(withUser))
				return

			default:
				http.Error(w, "Unknown authentication method", 500)
				return
			}
		})
	}
}

func withAdminSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// session and user exist at this point
		session := r.Context().Value(CtxSession).(*sessions.Session)
		user := r.Context().Value(CtxUser).(*db.User)
		if !user.IsAdmin {
			sessAddFlashW(session, []string{"you are not an admin"})
			sessLogSave(session, w, r)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type Response struct {
	// code is 200
	template string
	data     *templateData
	// code is 303
	redirect string
	flashN   []string // normal
	flashW   []string // warning
	// code is >= 400
	code int
	err  string
}

type (
	handlerAdmin func(r *http.Request) *Response
)

func respHandler(templateFS embed.FS, resolvePath func(string) string) func(next handlerAdmin) http.Handler {
	tmpl := template.Must(template.
		New("layout").
		Funcs(template.FuncMap(sprig.FuncMap())).
		Funcs(funcMap()).
		Funcs(template.FuncMap{"path": resolvePath}).
		ParseFS(templateFS, "*.tmpl", "**/*.tmpl"),
	)
	buffPool := sync.Pool{New: func() any { return new(bytes.Buffer) }}

	return func(next handlerAdmin) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := next(r)
			session, ok := r.Context().Value(CtxSession).(*sessions.Session)
			if ok {
				sessAddFlashN(session, resp.flashN)
				sessAddFlashW(session, resp.flashW)
				if err := session.Save(r, w); err != nil {
					http.Error(w, fmt.Sprintf("error saving session: %v", err), 500)
					return
				}
			}
			if resp.redirect != "" {
				http.Redirect(w, r, resolvePath(resp.redirect), http.StatusSeeOther)
				return
			}
			if resp.err != "" {
				http.Error(w, resp.err, resp.code)
				return
			}
			if resp.template == "" {
				http.Error(w, "useless handler return", 500)
				return
			}

			if resp.data == nil {
				resp.data = &templateData{}
			}
			resp.data.Version = gonic.Version
			if session != nil {
				resp.data.Flashes = session.Flashes()
				if err := session.Save(r, w); err != nil {
					http.Error(w, fmt.Sprintf("error saving session: %v", err), 500)
					return
				}
			}
			if user, ok := r.Context().Value(CtxUser).(*db.User); ok {
				resp.data.User = user
			}

			buff := buffPool.Get().(*bytes.Buffer)
			defer buffPool.Put(buff)
			buff.Reset()

			if err := tmpl.ExecuteTemplate(buff, resp.template, resp.data); err != nil {
				http.Error(w, fmt.Sprintf("executing template: %v", err), 500)
				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if resp.code != 0 {
				w.WriteHeader(resp.code)
			}
			if _, err := buff.WriteTo(w); err != nil {
				log.Printf("error writing to response buffer: %v\n", err)
			}
		})
	}
}

func respRaw(h http.HandlerFunc) http.Handler {
	return h // stub
}

type templateData struct {
	// common
	Flashes []interface{}
	User    *db.User
	Version string

	// home
	Stats                db.Stats
	RequestRoot          string
	RecentFolders        []*db.Album
	AllUsers             []*db.User
	LastScanTime         time.Time
	IsScanning           bool
	TranscodePreferences []*db.TranscodePreference
	TranscodeProfiles    []string

	CurrentLastFMAPIKey    string
	CurrentLastFMAPISecret string
	DefaultListenBrainzURL string
	SelectedUser           *db.User

	Podcasts              []*db.Podcast
	InternetRadioStations []*db.InternetRadioStation

	// avatar
	Avatar []byte

	// custom properties
	Props map[string]interface{}
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"str": func(in any) string {
			v, _ := json.Marshal(in)
			return string(v)
		},
		"noCache": func(in string) string {
			parsed, _ := url.Parse(in)
			params := parsed.Query()
			params.Set("v", gonic.Version)
			parsed.RawQuery = params.Encode()
			return parsed.String()
		},
		"date": func(in time.Time) string {
			return strings.ToLower(in.Format("Jan 02, 2006"))
		},
		"dateHuman": humanize.Time,
		"base64":    base64.StdEncoding.EncodeToString,
		"props": func(parent any, values ...any) map[string]any {
			if len(values)%2 != 0 {
				panic("uneven number of key/value pairs")
			}
			props := map[string]any{}
			for i := 0; i < len(values); i += 2 {
				k, v := fmt.Sprint(values[i]), values[i+1]
				props[k] = v
			}
			merged := map[string]any{}
			if structs.IsStruct(parent) {
				merged = structs.Map(parent)
			}
			merged["Props"] = props
			return merged
		},
	}
}

//  utilities

type FlashType string

const (
	FlashNormal  = FlashType("normal")
	FlashWarning = FlashType("warning")
)

type Flash struct {
	Message string
	Type    FlashType
}

// gob registrations are next to each other, in case there's more added later)
//
//nolint:gochecknoinits // for now I think it's nice that our types and their
func init() {
	gob.Register(&Flash{})
}

func sessAddFlashN(s *sessions.Session, messages []string) {
	sessAddFlash(s, messages, FlashNormal)
}

func sessAddFlashW(s *sessions.Session, messages []string) {
	sessAddFlash(s, messages, FlashWarning)
}

func sessAddFlash(s *sessions.Session, messages []string, flashT FlashType) {
	if len(messages) == 0 {
		return
	}
	for i, message := range messages {
		if i > 6 {
			break
		}
		s.AddFlash(Flash{
			Message: message,
			Type:    flashT,
		})
	}
}

func sessLogSave(s *sessions.Session, w http.ResponseWriter, r *http.Request) {
	if err := s.Save(r, w); err != nil {
		log.Printf("error saving session: %v\n", err)
	}
}

// validation

var (
	errValiNoUsername        = errors.New("please enter a username")
	errValiPasswordAllFields = errors.New("please enter the password twice")
	errValiPasswordsNotSame  = errors.New("passwords entered were not the same")
	errValiKeysAllFields     = errors.New("please enter the api key and secret")
)

func validateUsername(username string) error {
	if username == "" {
		return errValiNoUsername
	}
	return nil
}

func validatePasswords(pOne, pTwo string) error {
	if pOne == "" || pTwo == "" {
		return errValiPasswordAllFields
	}
	if !(pOne == pTwo) {
		return errValiPasswordsNotSame
	}
	return nil
}

func validateAPIKey(apiKey, secret string) error {
	if apiKey == "" || secret == "" {
		return errValiKeysAllFields
	}
	return nil
}

func serveOIDCCallback(dbc *db.DB, w http.ResponseWriter, r *http.Request, resolveProxyPath ProxyPathResolver) {
	if auth.GetOIDCAuthEndpoint() == "" {
		http.Error(w, "OIDC not configured", http.StatusInternalServerError)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	// Get state parameter (FIXME: should validate this for CSRF protection)
	state := r.URL.Query().Get("state")
	log.Printf("OIDC callback received: code=%s..., state=%s", code[:min(10, len(code))], state)

	tokenResp, err := auth.ExchangeCodeForTokens(r, code)
	if err != nil {
		log.Printf("Error exchanging code for tokens: %v", err)
		http.Error(w, "Failed to exchange code for tokens", http.StatusInternalServerError)
		return
	}

	claims, err := auth.ValidateIncomingJWT(tokenResp.IDToken)
	if err != nil {
		log.Printf("Error validating ID token: %v", err)
		http.Error(w, "Invalid ID token", http.StatusUnauthorized)
		return
	}

	session := r.Context().Value(CtxSession).(*sessions.Session)
	_, err = auth.HandleOIDCLogin(dbc, session, claims)
	if err != nil {
		log.Printf("Error handling OIDC login: %v", err)
		http.Error(w, fmt.Sprintf("Failed to handle OIDC login: %v", err), http.StatusInternalServerError)
		return
	}
	sessLogSave(session, w, r)

	http.Redirect(w, r, resolveProxyPath("/admin/home"), http.StatusSeeOther)
}
