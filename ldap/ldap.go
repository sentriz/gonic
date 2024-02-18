package ldap

import (
	"errors"
	"fmt"
	"log"

	"go.senan.xyz/gonic/db"

	"github.com/go-ldap/ldap/v3"
)

func CheckLDAPcreds(username string, password string, dbc *db.DB) (bool, error) {
	err := createUserFromLDAP(username, dbc)
	if err != nil {
		return false, err
	}

	ldapFQDN, err := dbc.GetSetting("ldap_fqdn")

	// This checks if LDAP is setup or not.
	if ldapFQDN == "" || err != nil {
		return false, err
	}

	// The configuration page wouldn't allow these setting to not be set
	// while LDAP is enabled (a FQDN/IP is set).
	ldapPort, _ := dbc.GetSetting("ldap_port")
	baseDN, _ := dbc.GetSetting("ldap_base_dn")
	tls, _ := dbc.GetSetting("ldap_tls")

	// Now, we can try to connect to the LDAP server.
	l, err := createLDAPconnection(tls, ldapFQDN, ldapPort)
	if err != nil {
		// Return a generic error.
		log.Println("Failed to connect to LDAP server:", err)
		return false, errors.New("failed to connect to LDAP server")
	}
	defer l.Close()

	// After we have a connection, let's try binding
	_, err = l.SimpleBind(&ldap.SimpleBindRequest{
		Username: fmt.Sprintf("uid=%s,%s", username, baseDN),
		Password: password,
	})

	if err == nil {
		return true, nil
	}

	log.Println("Failed to bind to LDAP server:", err)
	return false, nil
}

// Creates a user from creds
func createUserFromLDAP(username string, dbc *db.DB) error {
	user := dbc.GetUserByName(username)
	if user != nil {
		return nil
	}

	ldapFQDN, err := dbc.GetSetting("ldap_fqdn")

	// This is basically checking if LDAP is setup, if ldapFQDN isn't set we can
	// assume that the user hasn't configured LDAP.
	if ldapFQDN == "" {
		return nil
	} else if err != nil {
		return err
	}

	// The configuration page wouldn't allow these setting to not be set
	// while LDAP is enabled (a FQDN/IP is set).
	bindUID, _ := dbc.GetSetting("ldap_bind_user")
	bindPWD, _ := dbc.GetSetting("ldap_bind_user_password")
	ldapPort, _ := dbc.GetSetting("ldap_port")
	baseDN, _ := dbc.GetSetting("ldap_base_dn")
	filter, _ := dbc.GetSetting("ldap_filter")
	tls, _ := dbc.GetSetting("ldap_tls")

	// Now, we can try to connect to the LDAP server.
	l, err := createLDAPconnection(tls, ldapFQDN, ldapPort)
	if err != nil {
		// Return a generic error.
		return errors.New("failed to connect to LDAP server")
	}
	defer l.Close()

	// After we have a connection, let's try binding
	_, err = l.SimpleBind(&ldap.SimpleBindRequest{
		Username: fmt.Sprintf("uid=%s,%s", bindUID, baseDN),
		Password: bindPWD,
	})
	if err != nil {
		log.Println("Failed to bind to LDAP:", err)
		return errors.New("wrong username or password")
	}

	searchReq := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&%s(uid=%s))", filter, ldap.EscapeFilter(username)),
		[]string{"dn"},
		nil,
	)

	result, err := l.Search(searchReq)
	if err != nil {
		log.Println("failed to query LDAP server:", err)
	}

	switch len(result.Entries) {
	case 1:
		user := db.User{
			Name:     username,
			Password: "", // no password because we want auth to fail.
		}

		if err := dbc.Create(&user).Error; err != nil {
			log.Println("User created via LDAP:", username)
		}

		return nil
	case 0:
		return errors.New("invalid username")
	default:
		return errors.New("ambiguous user")
	}
}

// Creates a connection to an LDAP server.
func createLDAPconnection(tls string, fqdn string, port string) (*ldap.Conn, error) {
	protocol := "ldap"
	if tls == "true" {
		protocol = "ldaps"
	}

	// Now, we can try to connect to the LDAP server.
	l, err := ldap.DialURL(fmt.Sprintf("%s://%s:%s", protocol, fqdn, port))
	if err != nil {
		// Warn the server and return a generic error.
		log.Println("Failed to connect to LDAP server", err)
		return nil, err
	}

	return l, nil
}
