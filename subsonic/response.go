// from "sonicmonkey" by https://github.com/jeena/sonicmonkey/

package subsonic

import "encoding/xml"

var (
	apiVersion = "1.9.0"
	xmlns      = "http://subsonic.org/restapi"
)

type MetaResponse struct {
	XMLName   xml.Name `xml:"subsonic-response" json:"-"`
	*Response `json:"subsonic-response"`
}

type Response struct {
	Status       string         `xml:"status,attr"              json:"status"`
	Version      string         `xml:"version,attr"             json:"version"`
	XMLNS        string         `xml:"xmlns,attr"               json:"-"`
	Error        *Error         `xml:"error"                    json:"error,omitempty"`
	Albums       []*Album       `xml:"albumList2>album"         json:"albumList2,omitempty"`
	Album        *Album         `xml:"album"                    json:"album,omitempty"`
	Track        *Track         `xml:"song"                     json:"song,omitempty"`
	Indexes      *Indexes       `xml:"indexes"                  json:"indexes,omitempty"`
	Artists      []*Index       `xml:"artists>index"            json:"artists,omitempty"`
	Artist       *Artist        `xml:"artist"                   json:"artist,omitempty"`
	Directory    *Directory     `xml:"directory"                json:"directory,omitempty"`
	RandomTracks *RandomTracks  `xml:"randomSongs"              json:"randomSongs,omitempty"`
	MusicFolders []*MusicFolder `xml:"musicFolders>musicFolder" json:"musicFolders,omitempty"`
	Licence      *Licence       `xml:"license"                  json:"license,omitempty"`
}

type Error struct {
	Code    uint64 `xml:"code,attr"    json:"code"`
	Message string `xml:"message,attr" json:"message"`
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
