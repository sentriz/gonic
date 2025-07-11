package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/sessions"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/handlerutil"
)

// Global OIDC configuration for validation
var oidcConfig OIDCConfigOptions

// OIDC discovery and key structures
type OIDCDiscoveryConfig struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JwksURI               string `json:"jwks_uri"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
}

type JWKSResponse struct {
	Keys []JWK `json:"keys"`
}

type JWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	JwksURI      string
	Keys         map[string]*rsa.PublicKey
}

type AuthMethod string

type OIDCConfigOptions struct {
	Issuer        string
	ClientID      string
	ClientSecret  string // only for 'oidc'
	Keys          map[string]*rsa.PublicKey
	AuthEndpoint  string
	TokenEndpoint string
	HeaderName    string // only for 'oidc-forward'
	AdminRole     string
}

type FullOIDCConfig struct {
	*OIDCConfig
	AuthorizationEndpoint string
	TokenEndpoint         string
	AuthMethod            AuthMethod
	HeaderName            string
}

// The extra JWT claims understood by gonic
type CustomClaims struct {
	jwt.RegisteredClaims
	Name  string   `json:"name,omitempty"`
	Roles []string `json:"roles,omitempty"`
}

type OIDCTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

func SetOIDCConfig(config OIDCConfigOptions) {
	oidcConfig = config
}

func GetOIDCHeader() string {
	return oidcConfig.HeaderName
}

func GetOIDCAuthEndpoint() string {
	return oidcConfig.AuthEndpoint
}

func DebugPrintJWT(tokenString string) {
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, &CustomClaims{})
	if err != nil {
		log.Printf("JWT parsing error: %v", err)
		return
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		log.Printf("JWT claims parsing failed")
		return
	}

	debugPrintJWTHeader(token)
	debugPrintJWTClaims(claims)
}

func debugPrintJWTHeader(token *jwt.Token) {
	kid := ""
	if token.Header["kid"] != nil {
		kid = token.Header["kid"].(string)
	}

	log.Printf("JWT Header: alg=%s, kid=%s, typ=%s",
		token.Header["alg"], kid, token.Header["typ"])
}

func debugPrintJWTClaims(claims *CustomClaims) {
	var exp, iat int64
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Unix()
	}
	if claims.IssuedAt != nil {
		iat = claims.IssuedAt.Unix()
	}

	log.Printf("JWT Claims: iss=%s, sub=%s, aud=%v, exp=%d, iat=%d",
		claims.Issuer, claims.Subject, claims.Audience, exp, iat)
	if claims.Name != "" {
		log.Printf("JWT Name: %s", claims.Name)
	}
	if len(claims.Roles) > 0 {
		log.Printf("JWT Roles: %v", claims.Roles)
	}
}

func ValidateJWTToken(tokenString string, issuer, clientID string, keys map[string]*rsa.PublicKey) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kidInterface, ok := token.Header["kid"]
		if !ok {
			return nil, fmt.Errorf("no kid found in token header")
		}
		kid, ok := kidInterface.(string)
		if !ok {
			return nil, fmt.Errorf("kid is not a string")
		}
		rsaKey, exists := keys[kid]
		if !exists {
			return nil, fmt.Errorf("key ID %q not found in JWKS", kid)
		}

		return rsaKey, nil
	}, jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(clientID))

	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

func ValidateIncomingJWT(tokenString string) (*CustomClaims, error) {
	if oidcConfig.Issuer == "" || oidcConfig.ClientID == "" || oidcConfig.Keys == nil {
		return nil, fmt.Errorf("OIDC not configured")
	}
	return ValidateJWTToken(tokenString, oidcConfig.Issuer, oidcConfig.ClientID, oidcConfig.Keys)
}

func CheckUserIsAdmin(claims *CustomClaims) bool {
	if oidcConfig.AdminRole == "" || len(claims.Roles) == 0 {
		return false
	}

	for _, role := range claims.Roles {
		if role == oidcConfig.AdminRole {
			return true
		}
	}

	return false
}

func BuildOIDCAuthURL(authEndpoint string, r *http.Request) string {
	authURL, err := url.Parse(authEndpoint)
	if err != nil {
		log.Printf("Failed to parse auth endpoint: %v", err)
		return authEndpoint
	}

	baseURL := handlerutil.BaseURL(r)
	redirectURI := baseURL + "/admin/oidc/callback"

	state, err := GenerateRandomState()
	if err != nil {
		log.Printf("Failed to generate random state: %v", err)
		state = ""
	}

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", oidcConfig.ClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", "openid profile")
	params.Set("state", state)

	authURL.RawQuery = params.Encode()

	log.Printf("Built OIDC auth URL: %s", authURL.String())
	return authURL.String()
}

func GenerateRandomPassword() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random password: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func GenerateRandomState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("error generating random state: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func ExchangeCodeForTokens(r *http.Request, code string) (*OIDCTokenResponse, error) {
	baseURL := handlerutil.BaseURL(r)
	redirectURI := baseURL + "/admin/oidc/callback"

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", oidcConfig.ClientID)
	data.Set("client_secret", oidcConfig.ClientSecret)

	req, err := http.NewRequest("POST", oidcConfig.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp OIDCTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}

	return &tokenResp, nil
}

func BuildRedirectURI(r *http.Request) string {
	baseURL := handlerutil.BaseURL(r)
	return baseURL + "/admin/oidc/callback"
}

func HandleOIDCLogin(dbc *db.DB, sess *sessions.Session, claims *CustomClaims) (*db.User, error) {
	user := &db.User{}
	err := dbc.Where("oidc_subject = ?", claims.Subject).First(user).Error
	if err != nil {
		user, err = createOIDCUser(dbc, claims)
		if err != nil {
			return nil, err
		}
	} else {
		err = updateOIDCUserAdmin(dbc, user, claims)
		if err != nil {
			return nil, err
		}
	}

	sess.Values["user"] = user.ID
	return user, nil
}

func createOIDCUser(dbc *db.DB, claims *CustomClaims) (*db.User, error) {
	isAdmin := CheckUserIsAdmin(claims)

	password, err := GenerateRandomPassword()
	if err != nil {
		return nil, fmt.Errorf("error generating random password: %w", err)
	}

	name := claims.Name
	if name == "" {
		name = claims.Subject
	}
	user := &db.User{
		Name:        name,
		OIDCSubject: claims.Subject,
		Password:    password,
		IsAdmin:     isAdmin,
	}
	if err := dbc.Create(user).Error; err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	log.Printf("Created new OIDC user: %s (admin: %t)", user.Name, user.IsAdmin)
	return user, nil
}

func updateOIDCUserAdmin(dbc *db.DB, user *db.User, claims *CustomClaims) error {
	isAdmin := CheckUserIsAdmin(claims)

	if user.IsAdmin != isAdmin {
		user.IsAdmin = isAdmin
		if err := dbc.Save(user).Error; err != nil {
			return fmt.Errorf("error updating user: %w", err)
		}
		log.Printf("Updated OIDC user: %s (admin: %t)", user.Name, user.IsAdmin)
	}
	return nil
}

func ValidateOIDCIssuer(issuerURL string) (*OIDCDiscoveryConfig, error) {
	if issuerURL == "" {
		return nil, nil
	}

	discoveryURL := strings.TrimSuffix(issuerURL, "/") + "/.well-known/openid-configuration"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request to OIDC discovery URL: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to OIDC discovery URL %q: %w", discoveryURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC discovery URL %q returned status %d", discoveryURL, resp.StatusCode)
	}

	var config OIDCDiscoveryConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("parsing OIDC discovery response: %w", err)
	}

	if config.Issuer == "" || config.JwksURI == "" {
		return nil, fmt.Errorf("OIDC discovery response missing required fields (issuer or jwks_uri)")
	}

	return &config, nil
}

func ParseJWK(jwk JWK) (*rsa.PublicKey, error) {
	if jwk.Kty != "RSA" {
		return nil, fmt.Errorf("unsupported key type: %s", jwk.Kty)
	}

	// Decode the modulus (n)
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}
	n := new(big.Int).SetBytes(nBytes)

	// Decode the exponent (e)
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}

func DownloadJWKS(jwksURI string) (map[string]*rsa.PublicKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", jwksURI, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request to JWKS URI: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading JWKS from %q: %w", jwksURI, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS URI %q returned status %d", jwksURI, resp.StatusCode)
	}

	var jwksResp JWKSResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwksResp); err != nil {
		return nil, fmt.Errorf("parsing JWKS response: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, jwk := range jwksResp.Keys {
		if jwk.Use == "sig" || jwk.Use == "" { // Only use signing keys or unspecified
			pubKey, err := ParseJWK(jwk)
			if err != nil {
				log.Printf("warning: failed to parse JWK with kid %q: %v", jwk.Kid, err)
				continue
			}
			keys[jwk.Kid] = pubKey
		}
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no valid RSA keys found in JWKS")
	}

	return keys, nil
}

func ValidateOIDCConfig(issuerURL, clientID, clientSecret, clientSecretFile, headerName string, authMethod AuthMethod) (*FullOIDCConfig, error) {
	switch authMethod {
	case "password":
		return nil, nil
	case "oidc":
		// oidc method requires issuer URL, client ID, and client secret
		if issuerURL == "" {
			return nil, fmt.Errorf("oidc-issuer-url is required when auth-method is 'oidc'")
		}
		if clientID == "" {
			return nil, fmt.Errorf("oidc-client-id is required when auth-method is 'oidc'")
		}
		if clientSecret == "" && clientSecretFile == "" {
			return nil, fmt.Errorf("oidc-client-secret or oidc-client-secret-file is required when auth-method is 'oidc'")
		}
	case "oidc-forward":
		// oidc-forward method requires issuer URL, client ID, and header name
		if issuerURL == "" {
			return nil, fmt.Errorf("oidc-issuer-url is required when auth-method is 'oidc-forward'")
		}
		if clientID == "" {
			return nil, fmt.Errorf("oidc-client-id is required when auth-method is 'oidc-forward'")
		}
		if headerName == "" {
			return nil, fmt.Errorf("oidc-forward-header is required when auth-method is 'oidc-forward'")
		}
	}

	var finalClientSecret string
	if clientSecret != "" && clientSecretFile != "" {
		return nil, fmt.Errorf("cannot specify both oidc-client-secret and oidc-client-secret-file")
	}

	if clientSecret != "" {
		finalClientSecret = clientSecret
	} else if clientSecretFile != "" {
		secretBytes, err := os.ReadFile(clientSecretFile)
		if err != nil {
			return nil, fmt.Errorf("reading client secret file %q: %w", clientSecretFile, err)
		}
		finalClientSecret = strings.TrimSpace(string(secretBytes))
		if finalClientSecret == "" {
			return nil, fmt.Errorf("client secret file %q is empty", clientSecretFile)
		}
	}

	discoveryConfig, err := ValidateOIDCIssuer(issuerURL)
	if err != nil {
		return nil, fmt.Errorf("validating OIDC issuer: %w", err)
	}

	keys, err := DownloadJWKS(discoveryConfig.JwksURI)
	if err != nil {
		return nil, fmt.Errorf("downloading JWKS: %w", err)
	}

	return &FullOIDCConfig{
		OIDCConfig: &OIDCConfig{
			Issuer:       discoveryConfig.Issuer,
			ClientID:     clientID,
			ClientSecret: finalClientSecret,
			JwksURI:      discoveryConfig.JwksURI,
			Keys:         keys,
		},
		AuthorizationEndpoint: discoveryConfig.AuthorizationEndpoint,
		TokenEndpoint:         discoveryConfig.TokenEndpoint,
		AuthMethod:            authMethod,
		HeaderName:            headerName,
	}, nil
}
