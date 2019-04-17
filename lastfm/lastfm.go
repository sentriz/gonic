package lastfm

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var (
	baseURL = "http://ws.audioscrobbler.com/2.0/"
	client  = &http.Client{
		Timeout: 10 * time.Second,
	}
)

func getParamSignature(params url.Values, secret string) string {
	toHash := ""
	for k, v := range params {
		toHash += k
		toHash += v[0]
	}
	toHash += secret
	hash := md5.Sum([]byte(toHash))
	return hex.EncodeToString(hash[:])
}

func GetSession(apiKey, secret, token string) (string, error) {
	params := url.Values{}
	// the first 3 parameters here must be in alphabetical order
	params.Add("api_key", apiKey)
	params.Add("method", "auth.getSession")
	params.Add("token", token)
	params.Add("api_sig", getParamSignature(params, secret))
	req, _ := http.NewRequest("GET", baseURL, nil)
	req.URL.RawQuery = params.Encode()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error when making request to last.fm: %v", err)
	}
	defer resp.Body.Close()
	decoder := xml.NewDecoder(resp.Body)
	var lastfm LastFM
	err = decoder.Decode(&lastfm)
	if err != nil {
		return "", fmt.Errorf("error when decoding last.fm response: %v", err)
	}
	if lastfm.Error != nil {
		return "", fmt.Errorf("error when parsing last.fm response: %v", lastfm.Error.Value)
	}
	return lastfm.Session.Key, nil
}
