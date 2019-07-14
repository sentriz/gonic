package spec

import (
	"fmt"
	"time"
)

var (
	apiVersion = "1.9.0"
	xmlns      = "http://subsonic.org/restapi"
)

type Response struct {
	Status            string             `xml:"status,attr"   json:"status"`
	Version           string             `xml:"version,attr"  json:"version"`
	XMLNS             string             `xml:"xmlns,attr"    json:"-"`
	Error             *Error             `xml:"error"         json:"error,omitempty"`
	Albums            *Albums            `xml:"albumList"     json:"albumList,omitempty"`
	AlbumsTwo         *Albums            `xml:"albumList2"    json:"albumList2,omitempty"`
	Album             *Album             `xml:"album"         json:"album,omitempty"`
	Track             *TrackChild        `xml:"song"          json:"song,omitempty"`
	Indexes           *Indexes           `xml:"indexes"       json:"indexes,omitempty"`
	Artists           *Artists           `xml:"artists"       json:"artists,omitempty"`
	Artist            *Artist            `xml:"artist"        json:"artist,omitempty"`
	Directory         *Directory         `xml:"directory"     json:"directory,omitempty"`
	RandomTracks      *RandomTracks      `xml:"randomSongs"   json:"randomSongs,omitempty"`
	MusicFolders      *MusicFolders      `xml:"musicFolders"  json:"musicFolders,omitempty"`
	ScanStatus        *ScanStatus        `xml:"scanStatus"    json:"scanStatus,omitempty"`
	Licence           *Licence           `xml:"license"       json:"license,omitempty"`
	SearchResultTwo   *SearchResultTwo   `xml:"searchResult2" json:"searchResult2,omitempty"`
	SearchResultThree *SearchResultThree `xml:"searchResult3" json:"searchResult3,omitempty"`
}

func NewResponse() *Response {
	return &Response{
		Status:  "ok",
		XMLNS:   xmlns,
		Version: apiVersion,
	}
}

// spec errors:
//  0  a generic error
// 10  required parameter is missing
// 20  incompatible subsonic rest protocol version. client must upgrade
// 30  incompatible subsonic rest protocol version. server must upgrade
// 40  wrong username or password
// 41  token authentication not supported for ldap users
// 50  user is not authorized for the given operation
// 60  the trial period for the subsonic server is over
// 70  the requested data was not found

type Error struct {
	Code    int    `xml:"code,attr"    json:"code"`
	Message string `xml:"message,attr" json:"message"`
}

func NewError(code int, message string, a ...interface{}) *Response {
	return &Response{
		Status:  "failed",
		XMLNS:   xmlns,
		Version: apiVersion,
		Error: &Error{
			Code:    code,
			Message: fmt.Sprintf(message, a...),
		},
	}
}

type Albums struct {
	List []*Album `xml:"album" json:"album,omitempty"`
}

type Album struct {
	// common
	ID       int    `xml:"id,attr,omitempty"       json:"id"`
	CoverID  int    `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	ArtistID int    `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	Artist   string `xml:"artist,attr,omitempty"   json:"artist,omitempty"`
	// browsing by folder (getAlbumList)
	Title    string `xml:"title,attr,omitempty"  json:"title,omitempty"`
	Album    string `xml:"album,attr,omitempty"  json:"album,omitempty"`
	ParentID int    `xml:"parent,attr,omitempty" json:"parent,omitempty"`
	IsDir    bool   `xml:"isDir,attr,omitempty"  json:"isDir,omitempty"`
	// browsing by tags (getAlbumList2)
	Name       string        `xml:"name,attr,omitempty"      json:"name,omitempty"`
	TrackCount int           `xml:"songCount,attr,omitempty" json:"songCount,omitempty"`
	Duration   int           `xml:"duration,attr,omitempty"  json:"duration,omitempty"`
	Created    time.Time     `xml:"created,attr,omitempty"   json:"created,omitempty"`
	Tracks     []*TrackChild `xml:"song,omitempty"           json:"song,omitempty"`
}

type RandomTracks struct {
	Tracks []*TrackChild `xml:"song"        json:"song"`
}

type TrackChild struct {
	Album       string    `xml:"album,attr,omitempty"       json:"album,omitempty"`
	AlbumID     int       `xml:"albumId,attr,omitempty"     json:"albumId,omitempty"`
	Artist      string    `xml:"artist,attr,omitempty"      json:"artist,omitempty"`
	ArtistID    int       `xml:"artistId,attr,omitempty"    json:"artistId,omitempty"`
	Bitrate     int       `xml:"bitRate,attr,omitempty"     json:"bitRate,omitempty"`
	ContentType string    `xml:"contentType,attr,omitempty" json:"contentType,omitempty"`
	CoverID     int       `xml:"coverArt,attr,omitempty"    json:"coverArt,omitempty"`
	CreatedAt   time.Time `xml:"created,attr,omitempty"     json:"created,omitempty"`
	Duration    int       `xml:"duration,attr,omitempty"    json:"duration,omitempty"`
	Genre       string    `xml:"genre,attr,omitempty"       json:"genre,omitempty"`
	ID          int       `xml:"id,attr,omitempty"          json:"id,omitempty"`
	IsDir       bool      `xml:"isDir,attr,omitempty"       json:"isDir,omitempty"`
	IsVideo     bool      `xml:"isVideo,attr,omitempty"     json:"isVideo,omitempty"`
	ParentID    int       `xml:"parent,attr,omitempty"      json:"parent,omitempty"`
	Path        string    `xml:"path,attr,omitempty"        json:"path,omitempty"`
	Size        int       `xml:"size,attr,omitempty"        json:"size,omitempty"`
	Suffix      string    `xml:"suffix,attr,omitempty"      json:"suffix,omitempty"`
	Title       string    `xml:"title,attr,omitempty"       json:"title,omitempty"`
	TrackNumber int       `xml:"track,attr,omitempty"       json:"track,omitempty"`
	DiscNumber  int       `xml:"discNumber,attr,omitempty"  json:"discNumber,omitempty"`
	Type        string    `xml:"type,attr,omitempty"        json:"type,omitempty"`
}

type Artists struct {
	List []*Index `xml:"index,omitempty" json:"index,omitempty"`
}

type Artist struct {
	ID         int      `xml:"id,attr,omitempty"         json:"id"`
	Name       string   `xml:"name,attr,omitempty"       json:"name"`
	CoverID    int      `xml:"coverArt,attr,omitempty"   json:"coverArt,omitempty"`
	AlbumCount int      `xml:"albumCount,attr,omitempty" json:"albumCount,omitempty"`
	Albums     []*Album `xml:"album,omitempty"           json:"album,omitempty"`
}

type Indexes struct {
	LastModified int      `xml:"lastModified,attr,omitempty" json:"lastModified"`
	Index        []*Index `xml:"index,omitempty"             json:"index"`
}

type Index struct {
	Name    string    `xml:"name,attr,omitempty" json:"name"`
	Artists []*Artist `xml:"artist,omitempty"    json:"artist"`
}

type Directory struct {
	ID       int           `xml:"id,attr,omitempty"      json:"id"`
	Parent   int           `xml:"parent,attr,omitempty"  json:"parent"`
	Name     string        `xml:"name,attr,omitempty"    json:"name"`
	Starred  string        `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	Children []*TrackChild `xml:"child,omitempty"        json:"child,omitempty"`
}

type MusicFolders struct {
	List []*MusicFolder `xml:"musicFolder,omitempty" json:"musicFolder,omitempty"`
}

type MusicFolder struct {
	ID   int    `xml:"id,attr,omitempty"   json:"id,omitempty"`
	Name string `xml:"name,attr,omitempty" json:"name,omitempty"`
}

type Licence struct {
	Valid bool `xml:"valid,attr,omitempty" json:"valid,omitempty"`
}

type ScanStatus struct {
	Scanning bool `xml:"scanning,attr"        json:"scanning"`
	Count    int  `xml:"count,attr,omitempty" json:"count,omitempty"`
}

type SearchResultTwo struct {
	Artists []*Directory  `xml:"artist,omitempty" json:"artist,omitempty"`
	Albums  []*TrackChild `xml:"album,omitempty"  json:"album,omitempty"`
	Tracks  []*TrackChild `xml:"song,omitempty"   json:"song,omitempty"`
}

type SearchResultThree struct {
	Artists []*Artist     `xml:"artist,omitempty" json:"artist,omitempty"`
	Albums  []*Album      `xml:"album,omitempty"  json:"album,omitempty"`
	Tracks  []*TrackChild `xml:"song,omitempty"   json:"song,omitempty"`
}
