package subsonic

import "encoding/xml"

type Album struct {
	XMLName    xml.Name `xml:"album"          json:"-"`
	ID         uint     `xml:"id,attr"        json:"id"`
	Name       string   `xml:"name,attr"      json:"name"`
	ArtistID   uint     `xml:"artistId,attr"  json:"artistId"`
	ArtistName string   `xml:"artist,attr"    json:"artist"`
	SongCount  uint     `xml:"songCount,attr" json:"songCount"`
	Duration   uint     `xml:"duration,attr"  json:"duration"`
	CoverArt   string   `xml:"coverArt,attr"  json:"coverArt"`
	Created    string   `xml:"created,attr"   json:"created"`
	Songs      *[]*Song `xml:"song"           json:"song,omitempty"`
}

type RandomSongs struct {
	XMLName xml.Name `xml:"randomSongs" json:"-"`
	Songs   []*Song  `xml:"song"        json:"song"`
}

type Song struct {
	XMLName     xml.Name `xml:"song"             json:"-"`
	ID          uint     `xml:"id,attr"          json:"id"`
	Parent      uint     `xml:"parent,attr"      json:"parent"`
	Title       string   `xml:"title,attr"       json:"title"`
	Album       string   `xml:"album,attr"       json:"album"`
	Artist      string   `xml:"artist,attr"      json:"artist"`
	IsDir       bool     `xml:"isDir,attr"       json:"isDir"`
	CoverArt    string   `xml:"coverArt,attr"    json:"coverArt"`
	Created     string   `xml:"created,attr"     json:"created"`
	Duration    uint     `xml:"duration,attr"    json:"duration"`
	Genre       string   `xml:"genre,attr"       json:"genre"`
	BitRate     uint     `xml:"bitRate,attr"     json:"bitRate"`
	Size        uint     `xml:"size,attr"        json:"size"`
	Suffix      string   `xml:"suffix,attr"      json:"suffix"`
	ContentType string   `xml:"contentType,attr" json:"contentType"`
	IsVideo     bool     `xml:"isVideo,attr"     json:"isVideo"`
	Path        string   `xml:"path,attr"        json:"path"`
	AlbumID     uint     `xml:"albumId,attr"     json:"albumId"`
	ArtistID    uint     `xml:"artistId,attr"    json:"artistId"`
	TrackNo     uint     `xml:"track,attr"       json:"track"`
	Type        string   `xml:"type,attr"        json:"type"`
}

type Artist struct {
	XMLName    xml.Name `xml:"artist"          json:"-"`
	ID         uint     `xml:"id,attr"         json:"id"`
	Name       string   `xml:"name,attr"       json:"name"`
	CoverArt   string   `xml:"coverArt,attr"   json:"coverArt,omitempty"`
	AlbumCount uint     `xml:"albumCount,attr" json:"albumCount,omitempty"`
	Albums     []*Album `xml:"album,omitempty" json:"album,omitempty"`
}

type Indexes struct {
	LastModified uint      `xml:"lastModified,attr" json:"lastModified"`
	Index        *[]*Index `xml:"index"             json:"index"`
}

type Index struct {
	XMLName xml.Name  `xml:"index"     json:"-"`
	Name    string    `xml:"name,attr" json:"name"`
	Artists []*Artist `xml:"artist"    json:"artist"`
}

type Directory struct {
	XMLName  xml.Name `xml:"directory"              json:"-"`
	ID       uint     `xml:"id,attr"                json:"id"`
	Parent   uint     `xml:"parent,attr"            json:"parent"`
	Name     string   `xml:"name,attr"              json:"name"`
	Starred  string   `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	Children []Child  `xml:"child"                  json:"child"`
}

type Child struct {
	XMLName     xml.Name `xml:"child"                      json:"-"`
	ID          uint     `xml:"id,attr"                    json:"id,omitempty"`
	Parent      uint     `xml:"parent,attr"                json:"parent,omitempty"`
	Title       string   `xml:"title,attr"                 json:"title,omitempty"`
	IsDir       bool     `xml:"isDir,attr"                 json:"isDir,omitempty"`
	Album       string   `xml:"album,attr,omitempty"       json:"album,omitempty"`
	AlbumID     uint     `xml:"albumId,attr,omitempty"     json:"albumId,omitempty"`
	Artist      string   `xml:"artist,attr,omitempty"      json:"artist,omitempty"`
	ArtistID    uint     `xml:"artistId,attr,omitempty"    json:"artistId,omitempty"`
	Track       uint     `xml:"track,attr,omitempty"       json:"track,omitempty"`
	Year        uint     `xml:"year,attr,omitempty"        json:"year,omitempty"`
	Genre       string   `xml:"genre,attr,omitempty"       json:"genre,omitempty"`
	CoverArt    uint     `xml:"coverart,attr"              json:"coverArt,omitempty"`
	Size        uint     `xml:"size,attr,omitempty"        json:"size,omitempty"`
	ContentType string   `xml:"contentType,attr,omitempty" json:"contentType,omitempty"`
	Suffix      string   `xml:"suffix,attr,omitempty"      json:"suffix,omitempty"`
	Duration    uint     `xml:"duration,attr,omitempty"    json:"duration"`
	BitRate     uint     `xml:"bitRate,attr,omitempty"     json:"bitrate,omitempty"`
	Path        string   `xml:"path,attr,omitempty"        json:"path,omitempty"`
}
