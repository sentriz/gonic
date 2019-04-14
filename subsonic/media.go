package subsonic

import "time"

type Album struct {
	ID         uint      `xml:"id,attr"        json:"id"`
	Name       string    `xml:"name,attr"      json:"name"`
	ArtistID   uint      `xml:"artistId,attr"  json:"artistId"`
	Artist     string    `xml:"artist,attr"    json:"artist"`
	TrackCount uint      `xml:"songCount,attr" json:"songCount"`
	Duration   uint      `xml:"duration,attr"  json:"duration"`
	CoverID    uint      `xml:"coverArt,attr"  json:"coverArt"`
	Created    time.Time `xml:"created,attr"   json:"created"`
	Tracks     []*Track  `xml:"song"           json:"song,omitempty"`
}

type RandomTracks struct {
	Tracks []*Track `xml:"song"        json:"song"`
}

type Track struct {
	ID          uint      `xml:"id,attr"          json:"id"`
	Parent      uint      `xml:"parent,attr"      json:"parent"`
	Title       string    `xml:"title,attr"       json:"title"`
	Album       string    `xml:"album,attr"       json:"album"`
	Artist      string    `xml:"artist,attr"      json:"artist"`
	IsDir       bool      `xml:"isDir,attr"       json:"isDir"`
	CoverID     uint      `xml:"coverArt,attr"    json:"coverArt"`
	Created     time.Time `xml:"created,attr"     json:"created"`
	Duration    uint      `xml:"duration,attr"    json:"duration"`
	Genre       string    `xml:"genre,attr"       json:"genre"`
	BitRate     uint      `xml:"bitRate,attr"     json:"bitRate"`
	Size        uint      `xml:"size,attr"        json:"size"`
	Suffix      string    `xml:"suffix,attr"      json:"suffix"`
	ContentType string    `xml:"contentType,attr" json:"contentType"`
	IsVideo     bool      `xml:"isVideo,attr"     json:"isVideo"`
	Path        string    `xml:"path,attr"        json:"path"`
	AlbumID     uint      `xml:"albumId,attr"     json:"albumId"`
	ArtistID    uint      `xml:"artistId,attr"    json:"artistId"`
	TrackNo     uint      `xml:"track,attr"       json:"track"`
	Type        string    `xml:"type,attr"        json:"type"`
}

type Artist struct {
	ID         uint     `xml:"id,attr"         json:"id"`
	Name       string   `xml:"name,attr"       json:"name"`
	CoverID    uint     `xml:"coverArt,attr"   json:"coverArt,omitempty"`
	AlbumCount uint     `xml:"albumCount,attr" json:"albumCount,omitempty"`
	Albums     []*Album `xml:"album,omitempty" json:"album,omitempty"`
}

type Indexes struct {
	LastModified uint     `xml:"lastModified,attr" json:"lastModified"`
	Index        []*Index `xml:"index"             json:"index"`
}

type Index struct {
	Name    string    `xml:"name,attr" json:"name"`
	Artists []*Artist `xml:"artist"    json:"artist"`
}

type Directory struct {
	ID       uint    `xml:"id,attr"                json:"id"`
	Parent   uint    `xml:"parent,attr"            json:"parent"`
	Name     string  `xml:"name,attr"              json:"name"`
	Starred  string  `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	Children []Child `xml:"child"                  json:"child"`
}

type Child struct {
	ID          uint   `xml:"id,attr"                    json:"id,omitempty"`
	Parent      uint   `xml:"parent,attr"                json:"parent,omitempty"`
	Title       string `xml:"title,attr"                 json:"title,omitempty"`
	IsDir       bool   `xml:"isDir,attr"                 json:"isDir,omitempty"`
	Album       string `xml:"album,attr,omitempty"       json:"album,omitempty"`
	AlbumID     uint   `xml:"albumId,attr,omitempty"     json:"albumId,omitempty"`
	Artist      string `xml:"artist,attr,omitempty"      json:"artist,omitempty"`
	ArtistID    uint   `xml:"artistId,attr,omitempty"    json:"artistId,omitempty"`
	Track       uint   `xml:"track,attr,omitempty"       json:"track,omitempty"`
	Year        uint   `xml:"year,attr,omitempty"        json:"year,omitempty"`
	Genre       string `xml:"genre,attr,omitempty"       json:"genre,omitempty"`
	CoverID     uint   `xml:"coverArt,attr"              json:"coverArt,omitempty"`
	Size        uint   `xml:"size,attr,omitempty"        json:"size,omitempty"`
	ContentType string `xml:"contentType,attr,omitempty" json:"contentType,omitempty"`
	Suffix      string `xml:"suffix,attr,omitempty"      json:"suffix,omitempty"`
	Duration    uint   `xml:"duration,attr,omitempty"    json:"duration"`
	BitRate     uint   `xml:"bitRate,attr,omitempty"     json:"bitrate,omitempty"`
	Path        string `xml:"path,attr,omitempty"        json:"path,omitempty"`
}

type MusicFolder struct {
	ID   uint   `xml:"id,attr"   json:"id,omitempty"`
	Name string `xml:"name,attr" json:"name,omitempty"`
}

type Licence struct {
	Valid bool `xml:"valid,attr,omitempty" json:"valid,omitempty"`
}
