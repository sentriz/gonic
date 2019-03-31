package subsonic

import "encoding/xml"

type Album struct {
	XMLName    xml.Name `xml:"album"          json:"-"`
	Id         uint64   `xml:"id,attr"        json:"id"`
	Name       string   `xml:"name,attr"      json:"name"`
	ArtistId   uint64   `xml:"artistId,attr"  json:"artistId"`
	ArtistName string   `xml:"artist,attr"    json:"artist"`
	SongCount  uint64   `xml:"songCount,attr" json:"songCount"`
	Duration   uint64   `xml:"duration,attr"  json:"duration"`
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
	Id          uint64   `xml:"id,attr"          json:"id"`
	Parent      uint64   `xml:"parent,attr"      json:"parent"`
	Title       string   `xml:"title,attr"       json:"title"`
	Album       string   `xml:"album,attr"       json:"album"`
	Artist      string   `xml:"artist,attr"      json:"artist"`
	IsDir       bool     `xml:"isDir,attr"       json:"isDir"`
	CoverArt    string   `xml:"coverArt,attr"    json:"coverArt"`
	Created     string   `xml:"created,attr"     json:"created"`
	Duration    uint64   `xml:"duration,attr"    json:"duration"`
	Genre       string   `xml:"genre,attr"       json:"genre"`
	BitRate     uint64   `xml:"bitRate,attr"     json:"bitRate"`
	Size        uint64   `xml:"size,attr"        json:"size"`
	Suffix      string   `xml:"suffix,attr"      json:"suffix"`
	ContentType string   `xml:"contentType,attr" json:"contentType"`
	IsVideo     bool     `xml:"isVideo,attr"     json:"isVideo"`
	Path        string   `xml:"path,attr"        json:"path"`
	AlbumId     uint64   `xml:"albumId,attr"     json:"albumId"`
	ArtistId    uint64   `xml:"artistId,attr"    json:"artistId"`
	TrackNo     uint64   `xml:"track,attr"       json:"track"`
	Type        string   `xml:"type,attr"        json:"type"`
}

type Artist struct {
	XMLName    xml.Name `xml:"artist"          json:"-"`
	Id         uint64   `xml:"id,attr"         json:"id"`
	Name       string   `xml:"name,attr"       json:"name"`
	CoverArt   string   `xml:"coverArt,attr"   json:"coverArt"`
	AlbumCount uint64   `xml:"albumCount,attr" json:"albumCount"`
	Albums     []*Album `xml:"album,omitempty" json:"album,omitempty"`
}

type Indexes struct {
	LastModified uint64    `xml:"lastModified,attr" json:"lastModified"`
	Index        *[]*Index `xml:"index"             json:"index"`
}

type Index struct {
	XMLName xml.Name  `xml:"index"     json:"-"`
	Name    string    `xml:"name,attr" json:"name"`
	Artists []*Artist `xml:"artist"    json:"artist"`
}

type Directory struct {
	XMLName  xml.Name `xml:"directory"              json:"-"`
	Id       string   `xml:"id,attr"                json:"id"`
	Parent   string   `xml:"parent,attr"            json:"parent"`
	Name     string   `xml:"name,attr"              json:"name"`
	Starred  string   `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	Children []Child  `xml:"child"                  json:"child"`
}

type Child struct {
	XMLName     xml.Name `xml:child`
	Id          string   `xml:"id,attr"`
	Parent      string   `xml:"parent,attr"`
	Title       string   `xml:"title,attr"`
	IsDir       bool     `xml:"isDir,attr"`
	Album       string   `xml:"album,attr,omitempty"`
	Artist      string   `xml:"artist,attr,omitempty"`
	Track       uint64   `xml:"track,attr,omitempty"`
	Year        uint64   `xml:"year,attr,omitempty"`
	Genre       string   `xml:"genre,attr,omitempty"`
	CoverArt    uint64   `xml:"coverart,attr"`
	Size        uint64   `xml:"size,attr,omitempty"`
	ContentType string   `xml:"contentType,attr,omitempty"`
	Suffix      string   `xml:"suffix,attr,omitempty"`
	Duration    uint64   `xml:"duration,attr,omitempty"`
	BitRate     uint64   `xml:"bitRate,attr,omitempty"`
	Path        string   `xml:"path,attr,omitempty"`
}
