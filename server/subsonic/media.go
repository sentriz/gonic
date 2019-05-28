package subsonic

import (
	"time"
)

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
	Name       string    `xml:"name,attr,omitempty"      json:"name,omitempty"`
	TrackCount int       `xml:"songCount,attr,omitempty" json:"songCount,omitempty"`
	Duration   int       `xml:"duration,attr,omitempty"  json:"duration,omitempty"`
	Created    time.Time `xml:"created,attr,omitempty"   json:"created,omitempty"`
	Tracks     []*Track  `xml:"song,omitempty"           json:"song,omitempty"`
}

type RandomTracks struct {
	Tracks []*Track `xml:"song"        json:"song"`
}

type Track struct {
	ID            int       `xml:"id,attr,omitempty"          json:"id"`
	Parent        int       `xml:"parent,attr,omitempty"      json:"parent"`
	Title         string    `xml:"title,attr,omitempty"       json:"title"`
	Album         string    `xml:"album,attr,omitempty"       json:"album"`
	Artist        string    `xml:"artist,attr,omitempty"      json:"artist"`
	IsDir         bool      `xml:"isDir,attr,omitempty"       json:"isDir"`
	CoverID       int       `xml:"coverArt,attr,omitempty"    json:"coverArt"`
	CreatedAt     time.Time `xml:"created,attr,omitempty"     json:"created"`
	Duration      int       `xml:"duration,attr,omitempty"    json:"duration"`
	Genre         string    `xml:"genre,attr,omitempty"       json:"genre"`
	Bitrate       int       `xml:"bitRate,attr,omitempty"     json:"bitRate"`
	Size          int       `xml:"size,attr,omitempty"        json:"size"`
	Suffix        string    `xml:"suffix,attr,omitempty"      json:"suffix"`
	ContentType   string    `xml:"contentType,attr,omitempty" json:"contentType"`
	IsVideo       bool      `xml:"isVideo,attr,omitempty"     json:"isVideo"`
	Path          string    `xml:"path,attr,omitempty"        json:"path"`
	AlbumID       int       `xml:"albumId,attr,omitempty"     json:"albumId"`
	ArtistID int       `xml:"artistId,attr,omitempty"    json:"artistId"`
	TrackNumber   int       `xml:"track,attr,omitempty"       json:"track"`
	Type          string    `xml:"type,attr,omitempty"        json:"type"`
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
	ID       int      `xml:"id,attr,omitempty"      json:"id"`
	Parent   int      `xml:"parent,attr,omitempty"  json:"parent"`
	Name     string   `xml:"name,attr,omitempty"    json:"name"`
	Starred  string   `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	Children []*Child `xml:"child,omitempty"        json:"child"`
}

type Child struct {
	ID          int    `xml:"id,attr,omitempty"          json:"id,omitempty"`
	ParentID    int    `xml:"parent,attr,omitempty"      json:"parent,omitempty"`
	Title       string `xml:"title,attr,omitempty"       json:"title,omitempty"`
	IsDir       bool   `xml:"isDir,attr,omitempty"       json:"isDir,omitempty"`
	Album       string `xml:"album,attr,omitempty"       json:"album,omitempty"`
	AlbumID     int    `xml:"albumId,attr,omitempty"     json:"albumId,omitempty"`
	Artist      string `xml:"artist,attr,omitempty"      json:"artist,omitempty"`
	ArtistID    int    `xml:"artistId,attr,omitempty"    json:"artistId,omitempty"`
	Track       int    `xml:"track,attr,omitempty"       json:"track,omitempty"`
	Year        int    `xml:"year,attr,omitempty"        json:"year,omitempty"`
	Genre       string `xml:"genre,attr,omitempty"       json:"genre,omitempty"`
	CoverID     int    `xml:"coverArt,attr,omitempty"    json:"coverArt,omitempty"`
	Size        int    `xml:"size,attr,omitempty"        json:"size,omitempty"`
	ContentType string `xml:"contentType,attr,omitempty" json:"contentType,omitempty"`
	Suffix      string `xml:"suffix,attr,omitempty"      json:"suffix,omitempty"`
	Duration    int    `xml:"duration,attr,omitempty"    json:"duration,omitempty"`
	Bitrate     int    `xml:"bitRate,attr,omitempty"     json:"bitrate,omitempty"`
	Path        string `xml:"path,attr,omitempty"        json:"path,omitempty"`
	Type        string `xml:"type,attr,omitempty"        json:"type,omitempty"`
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
