package ldap

import (
	"errors"
	"fmt"
	"log"
	"time"

	"go.senan.xyz/gonic/db"

	"github.com/go-ldap/ldap/v3"
)

var ldapStore = make(LDAPStore)

// LDAPStore maps users to a cached password
type LDAPStore map[string]CachedLDAPpassword

// Add caches a password username set.
func (store *LDAPStore) Add(username, password string) {
	newStore := *store
	newStore[username] = CachedLDAPpassword{
		Password:  password,
		ExpiresAt: time.Now().Add(time.Hour * 8), // Keep the password valid for 8 hours.
	}

	store = &newStore
}

// IsValid checks if a user's password is stored in the cache and checks if a
// given password is valid.
func (store LDAPStore) IsValid(username, password string) bool {
	cached, ok := store[username]
	if !ok {
		return false
	}

	if cached.Password != password {
		return false
	}

	return cached.IsValid()
}

// CachedLDAPpassword stores an LDAP user's password and a time at which the
// server should no longer accept it.
type CachedLDAPpassword struct {
	Password  string
	ExpiresAt time.Time
}

func (password CachedLDAPpassword) IsValid() bool {
	return password.ExpiresAt.After(time.Now())
}

// Cofig stores the user's LDAP server options.
type Config struct {
	BindUser string
	BindPass string
	BaseDN   string

	Filter      string
	AdminFilter string

	FQDN string
	Port uint
	TLS  bool
}

func (c Config) IsSetup() bool {
	// This is basically checking if LDAP is setup, if ldapFQDN isn't set we can
	// assume that the user hasn't configured LDAP.
	return c.FQDN != ""
}

func CheckLDAPcreds(username string, password string, dbc *db.DB, config Config) (bool, error) {
	if !config.IsSetup() {
		return false, nil
	}

	if ldapStore.IsValid(username, password) {
		log.Println("Password authenticated via cache!")
		return true, nil
	}

	log.Println("Checking password against LDAP server ...")

	// Now, we can try to connect to the LDAP server.
	l, err := createLDAPconnection(config)
	if err != nil {
		// Return a generic error.
		log.Println("Failed to connect to LDAP server:", err)
		return false, errors.New("failed to connect to LDAP server")
	}
	defer l.Close()

	// Create the user if it doesn't exist on the database already.
	err = createUserFromLDAP(username, dbc, config, l)
	if err != nil {
		log.Println("Failed to create user from LDAP:", err)
		return false, err
	}

	// After we have a connection, let's try binding
	_, err = l.SimpleBind(&ldap.SimpleBindRequest{
		Username: fmt.Sprintf("uid=%s,%s", username, config.BaseDN),
		Password: password,
	})

	if err == nil {
		// Authentication was OK
		ldapStore.Add(username, password)
		return true, nil
	}

	log.Println("Failed to bind to LDAP server:", err)
	return false, nil
}

// Creates a user from creds
func createUserFromLDAP(username string, dbc *db.DB, config Config, l *ldap.Conn) error {
	user := dbc.GetUserByName(username)
	if user != nil {
		return nil
	}

	if !config.IsSetup() {
		return nil
	}

	isAdmin := doesLDAPAdminExist(username, config, l)
	log.Println(username, isAdmin)

	if doesLDAPUserExist(username, config, l) && !isAdmin {
		return errors.New("no such user")
	}

	newUser := db.User{
		Name:     username,
		Password: "", // no password because we want auth to fail.
		IsAdmin:  isAdmin,
	}

	err := dbc.Create(&newUser).Error
	if err != nil {
		return err
	}

	log.Println("User created via LDAP:", username)
	return nil
}

// doesLDAPAdminExist checks if an admin exists on the server.
func doesLDAPAdminExist(username string, config Config, l *ldap.Conn) bool {
	filter := fmt.Sprintf("(&(uid=%s)%s)", ldap.EscapeFilter(username), config.AdminFilter)

	searchReq := ldap.NewSearchRequest(
		config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{"dn"},
		nil,
	)

	result, err := l.Search(searchReq)
	if err != nil {
		log.Println("failed to query LDAP server:", err)
		return false
	}

	if len(result.Entries) == 1 {
		return true
	}

	return false
}

// doesLDAPUserExist checks if a user exists on the server.
func doesLDAPUserExist(username string, config Config, l *ldap.Conn) bool {
	filter := fmt.Sprintf("(&(uid=%s)%s)", ldap.EscapeFilter(username), config.Filter)

	searchReq := ldap.NewSearchRequest(
		config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{"dn"},
		nil,
	)

	result, err := l.Search(searchReq)
	if err != nil {
		log.Println("failed to query LDAP server:", err)
		return false
	}

	if len(result.Entries) == 1 {
		return true
	}

	return false
}

// Creates a connection to an LDAP server.
func createLDAPconnection(config Config) (*ldap.Conn, error) {
	protocol := "ldap"
	if config.TLS {
		protocol = "ldaps"
	}

	// Now, we can try to connect to the LDAP server.
	l, err := ldap.DialURL(fmt.Sprintf("%s://%s:%d", protocol, config.FQDN, config.Port))
	if err != nil {
		// Warn the server and return the error.
		log.Println("Failed to connect to LDAP server", err)
		return nil, err
	}

	// After we have a connection, let's try binding
	_, err = l.SimpleBind(&ldap.SimpleBindRequest{
		Username: fmt.Sprintf("uid=%s,%s", config.BindUser, config.BaseDN),
		Password: config.BindPass,
	})
	if err != nil {
		log.Println("Failed to bind to LDAP:", err)
		return nil, errors.New("wrong username or password")
	}

	return l, nil
}
