package subsonic

import "time"

type Album struct {
	ID         int       `xml:"id,attr"        json:"id"`
	Name       string    `xml:"name,attr"      json:"name"`
	ArtistID   int       `xml:"artistId,attr"  json:"artistId"`
	Artist     string    `xml:"artist,attr"    json:"artist"`
	TrackCount int       `xml:"songCount,attr" json:"songCount"`
	Duration   int       `xml:"duration,attr"  json:"duration"`
	CoverID    int       `xml:"coverArt,attr"  json:"coverArt"`
	Created    time.Time `xml:"created,attr"   json:"created"`
	Tracks     []*Track  `xml:"song"           json:"song,omitempty"`
}

type RandomTracks struct {
	Tracks []*Track `xml:"song"        json:"song"`
}

type Track struct {
	ID          int       `xml:"id,attr"          json:"id"`
	Parent      int       `xml:"parent,attr"      json:"parent"`
	Title       string    `xml:"title,attr"       json:"title"`
	Album       string    `xml:"album,attr"       json:"album"`
	Artist      string    `xml:"artist,attr"      json:"artist"`
	IsDir       bool      `xml:"isDir,attr"       json:"isDir"`
	CoverID     int       `xml:"coverArt,attr"    json:"coverArt"`
	Created     time.Time `xml:"created,attr"     json:"created"`
	Duration    int       `xml:"duration,attr"    json:"duration"`
	Genre       string    `xml:"genre,attr"       json:"genre"`
	BitRate     int       `xml:"bitRate,attr"     json:"bitRate"`
	Size        int       `xml:"size,attr"        json:"size"`
	Suffix      string    `xml:"suffix,attr"      json:"suffix"`
	ContentType string    `xml:"contentType,attr" json:"contentType"`
	IsVideo     bool      `xml:"isVideo,attr"     json:"isVideo"`
	Path        string    `xml:"path,attr"        json:"path"`
	AlbumID     int       `xml:"albumId,attr"     json:"albumId"`
	ArtistID    int       `xml:"artistId,attr"    json:"artistId"`
	TrackNo     int       `xml:"track,attr"       json:"track"`
	Type        string    `xml:"type,attr"        json:"type"`
}

type Artist struct {
	ID         int      `xml:"id,attr"         json:"id"`
	Name       string   `xml:"name,attr"       json:"name"`
	CoverID    int      `xml:"coverArt,attr"   json:"coverArt,omitempty"`
	AlbumCount int      `xml:"albumCount,attr" json:"albumCount,omitempty"`
	Albums     []*Album `xml:"album,omitempty" json:"album,omitempty"`
}

type Indexes struct {
	LastModified int      `xml:"lastModified,attr" json:"lastModified"`
	Index        []*Index `xml:"index"             json:"index"`
}

type Index struct {
	Name    string    `xml:"name,attr" json:"name"`
	Artists []*Artist `xml:"artist"    json:"artist"`
}

type Directory struct {
	ID       int     `xml:"id,attr"                json:"id"`
	Parent   int     `xml:"parent,attr"            json:"parent"`
	Name     string  `xml:"name,attr"              json:"name"`
	Starred  string  `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	Children []Child `xml:"child"                  json:"child"`
}

type Child struct {
	ID          int    `xml:"id,attr"                    json:"id,omitempty"`
	Parent      int    `xml:"parent,attr"                json:"parent,omitempty"`
	Title       string `xml:"title,attr"                 json:"title,omitempty"`
	IsDir       bool   `xml:"isDir,attr"                 json:"isDir,omitempty"`
	Album       string `xml:"album,attr,omitempty"       json:"album,omitempty"`
	AlbumID     int    `xml:"albumId,attr,omitempty"     json:"albumId,omitempty"`
	Artist      string `xml:"artist,attr,omitempty"      json:"artist,omitempty"`
	ArtistID    int    `xml:"artistId,attr,omitempty"    json:"artistId,omitempty"`
	Track       int    `xml:"track,attr,omitempty"       json:"track,omitempty"`
	Year        int    `xml:"year,attr,omitempty"        json:"year,omitempty"`
	Genre       string `xml:"genre,attr,omitempty"       json:"genre,omitempty"`
	CoverID     int    `xml:"coverArt,attr"              json:"coverArt,omitempty"`
	Size        int    `xml:"size,attr,omitempty"        json:"size,omitempty"`
	ContentType string `xml:"contentType,attr,omitempty" json:"contentType,omitempty"`
	Suffix      string `xml:"suffix,attr,omitempty"      json:"suffix,omitempty"`
	Duration    int    `xml:"duration,attr,omitempty"    json:"duration"`
	BitRate     int    `xml:"bitRate,attr,omitempty"     json:"bitrate,omitempty"`
	Path        string `xml:"path,attr,omitempty"        json:"path,omitempty"`
}

type MusicFolder struct {
	ID   int    `xml:"id,attr"   json:"id,omitempty"`
	Name string `xml:"name,attr" json:"name,omitempty"`
}

type Licence struct {
	Valid bool `xml:"valid,attr,omitempty" json:"valid,omitempty"`
}
