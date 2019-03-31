// from "sonicmonkey" by https://github.com/jeena/sonicmonkey/

package subsonic

import (
	"encoding/xml"
)

var (
	apiVersion = "1.10.0"
	xmlns      = "http://subsonic.org/restapi"
)

type Response struct {
	XMLName        xml.Name     `xml:"subsonic-response" json:"-"`
	Status         string       `xml:"status,attr"       json:"status"`
	Version        string       `xml:"version,attr"      json:"version"`
	XMLNS          string       `xml:"xmlns,attr"        json:"xmlns"`
	Error          *Error       `xml:"error"             json:"error,omitempty"`
	AlbumList2     *[]*Album    `xml:"albumList2>album"  json:"album,omitempty"`
	Album          *Album       `xml:"album"             json:"album,omitempty"`
	Song           *Song        `xml:"song"              json:"song,omitempty"`
	Indexes        *Indexes     `xml:"indexes"           json:"indexes,omitempty"`
	Artists        *[]*Index    `xml:"artists>index"     json:"artists,omitempty"`
	Artist         *Artist      `xml:"artist"            json:"artist,omitempty"`
	MusicDirectory *Directory   `xml:"directory"         json:"directory,omitempty"`
	RandomSongs    *RandomSongs `xml:"randomSongs"       json:"randomSongs,omitempty"`
}

type Error struct {
	XMLName xml.Name `xml:"error"        json:"-"`
	Code    uint64   `xml:"code,attr"    json:"code"`
	Message string   `xml:"message,attr" json:"message"`
}

func NewResponse() *Response {
	return &Response{
		Status:  "ok",
		XMLNS:   xmlns,
		Version: apiVersion,
	}
}

func NewError(code uint64, message string) *Response {
	return &Response{
		Status:  "failed",
		XMLNS:   xmlns,
		Version: apiVersion,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
}
