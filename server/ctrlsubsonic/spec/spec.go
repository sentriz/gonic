package spec

import (
	"fmt"
	"time"

	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/version"
)

const (
	apiVersion = "1.15.0"
	xmlns      = "http://subsonic.org/restapi"
)

type Response struct {
	Status            string             `xml:"status,attr"       json:"status"`
	Version           string             `xml:"version,attr"      json:"version"`
	XMLNS             string             `xml:"xmlns,attr"        json:"-"`
	Type              string             `xml:"type,attr"         json:"type"`
	GonicVersion      string             `xml:"gonicVersion,attr" json:"gonicVersion"`
	Error             *Error             `xml:"error"             json:"error,omitempty"`
	Albums            *Albums            `xml:"albumList"         json:"albumList,omitempty"`
	AlbumsTwo         *Albums            `xml:"albumList2"        json:"albumList2,omitempty"`
	Album             *Album             `xml:"album"             json:"album,omitempty"`
	Track             *TrackChild        `xml:"song"              json:"song,omitempty"`
	Indexes           *Indexes           `xml:"indexes"           json:"indexes,omitempty"`
	Artists           *Artists           `xml:"artists"           json:"artists,omitempty"`
	Artist            *Artist            `xml:"artist"            json:"artist,omitempty"`
	Directory         *Directory         `xml:"directory"         json:"directory,omitempty"`
	RandomTracks      *RandomTracks      `xml:"randomSongs"       json:"randomSongs,omitempty"`
	TracksByGenre     *TracksByGenre     `xml:"songsByGenre"      json:"songsByGenre,omitempty"`
	MusicFolders      *MusicFolders      `xml:"musicFolders"      json:"musicFolders,omitempty"`
	ScanStatus        *ScanStatus        `xml:"scanStatus"        json:"scanStatus,omitempty"`
	Licence           *Licence           `xml:"license"           json:"license,omitempty"`
	SearchResultTwo   *SearchResultTwo   `xml:"searchResult2"     json:"searchResult2,omitempty"`
	SearchResultThree *SearchResultThree `xml:"searchResult3"     json:"searchResult3,omitempty"`
	User              *User              `xml:"user"              json:"user,omitempty"`
	Playlists         *Playlists         `xml:"playlists"         json:"playlists,omitempty"`
	Playlist          *Playlist          `xml:"playlist"          json:"playlist,omitempty"`
	ArtistInfo        *ArtistInfo        `xml:"artistInfo"        json:"artistInfo,omitempty"`
	ArtistInfoTwo     *ArtistInfo        `xml:"artistInfo2"       json:"artistInfo2,omitempty"`
	Genres            *Genres            `xml:"genres"            json:"genres,omitempty"`
	PlayQueue         *PlayQueue         `xml:"playQueue"         json:"playQueue,omitempty"`
	JukeboxStatus     *JukeboxStatus     `xml:"jukeboxStatus"     json:"jukeboxStatus,omitempty"`
	JukeboxPlaylist   *JukeboxPlaylist   `xml:"jukeboxPlaylist"   json:"jukeboxPlaylist,omitempty"`
	Podcasts          *Podcasts          `xml:"podcasts"         json:"podcasts,omitempty"`
}

func NewResponse() *Response {
	return &Response{
		Status:       "ok",
		XMLNS:        xmlns,
		Version:      apiVersion,
		Type:         version.NAME,
		GonicVersion: version.VERSION,
	}
}

// Error represents a typed error
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
		Type:         version.NAME,
		GonicVersion: version.VERSION,
	}
}

type Albums struct {
	List []*Album `xml:"album" json:"album"`
}

type Album struct {
	// common
	ID       *specid.ID `xml:"id,attr,omitempty"       json:"id"`
	CoverID  *specid.ID `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	ArtistID *specid.ID `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	Artist   string     `xml:"artist,attr,omitempty"   json:"artist,omitempty"`
	// browsing by folder (eg. getAlbumList)
	Title    string     `xml:"title,attr,omitempty"  json:"title"`
	Album    string     `xml:"album,attr,omitempty"  json:"album"`
	ParentID *specid.ID `xml:"parent,attr,omitempty" json:"parent,omitempty"`
	IsDir    bool       `xml:"isDir,attr,omitempty"  json:"isDir,omitempty"`
	// browsing by tags (eg. getAlbumList2)
	Name       string        `xml:"name,attr"              json:"name"`
	TrackCount int           `xml:"songCount,attr"         json:"songCount"`
	Duration   int           `xml:"duration,attr"          json:"duration"`
	Created    time.Time     `xml:"created,attr,omitempty" json:"created,omitempty"`
	Genre      string        `xml:"genre,attr,omitempty"   json:"genre,omitempty"`
	Year       int           `xml:"year,attr,omitempty"    json:"year,omitempty"`
	Tracks     []*TrackChild `xml:"song,omitempty"         json:"song,omitempty"`
}

type RandomTracks struct {
	List []*TrackChild `xml:"song" json:"song"`
}

type TracksByGenre struct {
	List []*TrackChild `xml:"song" json:"song"`
}

type TrackChild struct {
	Album       string     `xml:"album,attr,omitempty"       json:"album,omitempty"`
	AlbumID     *specid.ID `xml:"albumId,attr,omitempty"     json:"albumId,omitempty"`
	Artist      string     `xml:"artist,attr,omitempty"      json:"artist,omitempty"`
	ArtistID    *specid.ID `xml:"artistId,attr,omitempty"    json:"artistId,omitempty"`
	Bitrate     int        `xml:"bitRate,attr,omitempty"     json:"bitRate,omitempty"`
	ContentType string     `xml:"contentType,attr,omitempty" json:"contentType,omitempty"`
	CoverID     *specid.ID `xml:"coverArt,attr,omitempty"    json:"coverArt,omitempty"`
	CreatedAt   time.Time  `xml:"created,attr,omitempty"     json:"created,omitempty"`
	Duration    int        `xml:"duration,attr,omitempty"    json:"duration,omitempty"`
	Genre       string     `xml:"genre,attr,omitempty"       json:"genre,omitempty"`
	ID          *specid.ID `xml:"id,attr,omitempty"          json:"id,omitempty"`
	IsDir       bool       `xml:"isDir,attr"                 json:"isDir"`
	IsVideo     bool       `xml:"isVideo,attr"               json:"isVideo"`
	ParentID    *specid.ID `xml:"parent,attr,omitempty"      json:"parent,omitempty"`
	Path        string     `xml:"path,attr,omitempty"        json:"path,omitempty"`
	Size        int        `xml:"size,attr,omitempty"        json:"size,omitempty"`
	Suffix      string     `xml:"suffix,attr,omitempty"      json:"suffix,omitempty"`
	Title       string     `xml:"title,attr"                 json:"title"`
	TrackNumber int        `xml:"track,attr,omitempty"       json:"track,omitempty"`
	DiscNumber  int        `xml:"discNumber,attr,omitempty"  json:"discNumber,omitempty"`
	Type        string     `xml:"type,attr,omitempty"        json:"type,omitempty"`
	Year        int        `xml:"year,attr,omitempty"        json:"year,omitempty"`
}

type Artists struct {
	IgnoredArticles string   `xml:"ignoredArticles,attr" json:"ignoredArticles"`
	List            []*Index `xml:"index"                json:"index"`
}

type Artist struct {
	ID         *specid.ID `xml:"id,attr,omitempty"       json:"id"`
	Name       string     `xml:"name,attr"               json:"name"`
	CoverID    *specid.ID `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	AlbumCount int        `xml:"albumCount,attr"         json:"albumCount"`
	Albums     []*Album   `xml:"album,omitempty"         json:"album,omitempty"`
}

type Indexes struct {
	LastModified    int      `xml:"lastModified,attr,omitempty" json:"lastModified"`
	IgnoredArticles string   `xml:"ignoredArticles,attr"        json:"ignoredArticles"`
	Index           []*Index `xml:"index"                       json:"index"`
}

type Index struct {
	Name    string    `xml:"name,attr,omitempty" json:"name"`
	Artists []*Artist `xml:"artist"              json:"artist"`
}

type Directory struct {
	ID       *specid.ID    `xml:"id,attr,omitempty"      json:"id"`
	ParentID *specid.ID    `xml:"parent,attr,omitempty"  json:"parent,omitempty"`
	Name     string        `xml:"name,attr,omitempty"    json:"name"`
	Starred  string        `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	Children []*TrackChild `xml:"child,omitempty"        json:"child,omitempty"`
}

type MusicFolders struct {
	List []*MusicFolder `xml:"musicFolder" json:"musicFolder"`
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

type User struct {
	Username            string `xml:"username,attr"            json:"username"`
	ScrobblingEnabled   bool   `xml:"scrobblingEnabled,attr"   json:"scrobblingEnabled"`
	AdminRole           bool   `xml:"adminRole,attr"           json:"adminRole"`
	SettingsRole        bool   `xml:"settingsRole,attr"        json:"settingsRole"`
	DownloadRole        bool   `xml:"downloadRole,attr"        json:"downloadRole"`
	UploadRole          bool   `xml:"uploadRole,attr"          json:"uploadRole"`
	PlaylistRole        bool   `xml:"playlistRole,attr"        json:"playlistRole"`
	CoverArtRole        bool   `xml:"coverArtRole,attr"        json:"coverArtRole"`
	CommentRole         bool   `xml:"commentRole,attr"         json:"commentRole"`
	PodcastRole         bool   `xml:"podcastRole,attr"         json:"podcastRole"`
	StreamRole          bool   `xml:"streamRole,attr"          json:"streamRole"`
	JukeboxRole         bool   `xml:"jukeboxRole,attr"         json:"jukeboxRole"`
	ShareRole           bool   `xml:"shareRole,attr"           json:"shareRole"`
	VideoConversionRole bool   `xml:"videoConversionRole,attr" json:"videoConversionRole"`
	Folder              []int  `xml:"folder,attr"              json:"folder"`
}

type Playlists struct {
	List []*Playlist `xml:"playlist" json:"playlist"`
}

type Playlist struct {
	ID        int           `xml:"id,attr"        json:"id"`
	Name      string        `xml:"name,attr"      json:"name"`
	Comment   string        `xml:"comment,attr"   json:"comment"`
	Owner     string        `xml:"owner,attr"     json:"owner"`
	SongCount int           `xml:"songCount,attr" json:"songCount"`
	Created   time.Time     `xml:"created,attr"   json:"created"`
	Duration  int           `xml:"duration,attr"  json:"duration,omitempty"`
	Public    bool          `xml:"public,attr"    json:"public,omitempty"`
	List      []*TrackChild `xml:"entry"          json:"entry"`
}

type SimilarArtist struct {
	ID         *specid.ID `xml:"id,attr"                   json:"id"`
	Name       string     `xml:"name,attr"                 json:"name"`
	AlbumCount int        `xml:"albumCount,attr,omitempty" json:"albumCount,omitempty"`
}

type ArtistInfo struct {
	Biography      string           `xml:"biography"               json:"biography"`
	MusicBrainzID  string           `xml:"musicBrainzId"           json:"musicBrainzId"`
	LastFMURL      string           `xml:"lastFmUrl"               json:"lastFmUrl"`
	SmallImageURL  string           `xml:"smallImageUrl"           json:"smallImageUrl"`
	MediumImageURL string           `xml:"mediumImageUrl"          json:"mediumImageUrl"`
	LargeImageURL  string           `xml:"largeImageUrl"           json:"largeImageUrl"`
	SimilarArtist  []*SimilarArtist `xml:"similarArtist,omitempty" json:"similarArtist,omitempty"`
}

type Genres struct {
	List []*Genre `xml:"genre" json:"genre"`
}

type Genre struct {
	Name       string `xml:",chardata"                 json:"value"`
	SongCount  int    `xml:"songCount,attr,omitempty"  json:"songCount,omitempty"`
	AlbumCount int    `xml:"albumCount,attr,omitempty" json:"albumCount,omitempty"`
}

type PlayQueue struct {
	Current   *specid.ID    `xml:"current,attr,omitempty"  json:"current,omitempty"`
	Position  int           `xml:"position,attr,omitempty" json:"position,omitempty"`
	Username  string        `xml:"username,attr"           json:"username"`
	Changed   time.Time     `xml:"changed,attr"            json:"changed"`
	ChangedBy string        `xml:"changedBy,attr"          json:"changedBy"`
	List      []*TrackChild `xml:"entry,omitempty"         json:"entry,omitempty"`
}

type JukeboxStatus struct {
	CurrentIndex int     `xml:"currentIndex,attr" json:"currentIndex"`
	Playing      bool    `xml:"playing,attr"      json:"playing"`
	Gain         float64 `xml:"gain,attr"         json:"gain"`
	Position     int     `xml:"position,attr"     json:"position"`
}

type JukeboxPlaylist struct {
	List []*TrackChild `xml:"entry,omitempty" json:"entry,omitempty"`
	JukeboxStatus
}

type Podcasts struct {
	List []*PodcastChannel `xml:"channel" json:"channel"`
}

type PodcastChannel struct {
	ID               *specid.ID        `xml:"id,attr" json:"id"`
	URL              string           `xml:"url,attr" json:"url"`
	Title            string           `xml:"title,attr" json:"title"`
	Description      string           `xml:"description,attr" json:"description"`
	CoverArt         *specid.ID        `xml:"coverArt,attr" json:"coverArt,omitempty"`
	OriginalImageURL string           `xml:"originalImageUrl,attr" json:"originalImageUrl,omitempty"`
	Status           string           `xml:"status,attr" json:"status"`
	Episode          []*PodcastEpisode `xml:"episode" json:"episode,omitempty"`
}

type PodcastEpisode struct {
	ID          *specid.ID `xml:"id,attr" json:"id"`
	StreamID    *specid.ID `xml:"streamId,attr" json:"streamId"`
	ChannelID   *specid.ID `xml:"channelId,attr" json:"channelId"`
	Title       string    `xml:"title,attr" json:"title"`
	Description string    `xml:"description,attr" json:"description"`
	PublishDate time.Time `xml:"publishDate,attr" json:"publishDate"`
	Status      string    `xml:"status,attr" json:"status"`
	Parent      string    `xml:"parent,attr" json:"parent"`
	IsDir       bool      `xml:"isDir,attr" json:"isDir"`
	Year        int       `xml:"year,attr" json:"year"`
	Genre       string    `xml:"genre,attr" json:"genre"`
	CoverArt    *specid.ID `xml:"coverArt,attr" json:"coverArt"`
	Size        int       `xml:"size,attr" json:"size"`
	ContentType string    `xml:"contentType,attr" json:"contentType"`
	Suffix      string    `xml:"suffix,attr" json:"suffix"`
	Duration    int       `xml:"duration,attr" json:"duration"`
	BitRate     int       `xml:"bitRate,attr" json:"bitrate"`
	Path        string    `xml:"path,attr" json:"path"`
}
