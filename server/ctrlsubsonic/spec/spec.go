package spec

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
	"go.senan.xyz/gonic"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

// https://web.archive.org/web/20220707025402/https://www.subsonic.org/pages/api.jsp

const (
	apiVersion = "1.15.0"
	xmlns      = "http://subsonic.org/restapi"
)

type SubsonicResponse struct {
	Response Response `xml:"subsonic-response"       json:"subsonic-response"`
}

type Response struct {
	Status  string `xml:"status,attr"           json:"status"`
	Version string `xml:"version,attr"          json:"version"`
	XMLNS   string `xml:"xmlns,attr"            json:"-"`

	// https://opensubsonic.netlify.app/docs/responses/subsonic-response/
	Type                   string                  `xml:"type,attr"              json:"type"`
	ServerVersion          string                  `xml:"serverVersion,attr"     json:"serverVersion"`
	OpenSubsonic           bool                    `xml:"openSubsonic,attr"      json:"openSubsonic"`
	OpenSubsonicExtensions *OpenSubsonicExtensions `xml:"openSubsonicExtensions" json:"openSubsonicExtensions,omitempty"`

	Error                 *Error                 `xml:"error"                 json:"error,omitempty"`
	Albums                *Albums                `xml:"albumList"             json:"albumList,omitempty"`
	AlbumsTwo             *Albums                `xml:"albumList2"            json:"albumList2,omitempty"`
	Album                 *Album                 `xml:"album"                 json:"album,omitempty"`
	Track                 *TrackChild            `xml:"song"                  json:"song,omitempty"`
	Indexes               *Indexes               `xml:"indexes"               json:"indexes,omitempty"`
	Artists               *Artists               `xml:"artists"               json:"artists,omitempty"`
	Artist                *Artist                `xml:"artist"                json:"artist,omitempty"`
	Directory             *Directory             `xml:"directory"             json:"directory,omitempty"`
	RandomTracks          *RandomTracks          `xml:"randomSongs"           json:"randomSongs,omitempty"`
	TracksByGenre         *TracksByGenre         `xml:"songsByGenre"          json:"songsByGenre,omitempty"`
	MusicFolders          *MusicFolders          `xml:"musicFolders"          json:"musicFolders,omitempty"`
	ScanStatus            *ScanStatus            `xml:"scanStatus"            json:"scanStatus,omitempty"`
	Licence               *Licence               `xml:"license"               json:"license,omitempty"`
	SearchResultTwo       *SearchResultTwo       `xml:"searchResult2"         json:"searchResult2,omitempty"`
	SearchResultThree     *SearchResultThree     `xml:"searchResult3"         json:"searchResult3,omitempty"`
	User                  *User                  `xml:"user"                  json:"user,omitempty"`
	Playlists             *Playlists             `xml:"playlists"             json:"playlists,omitempty"`
	Playlist              *Playlist              `xml:"playlist"              json:"playlist,omitempty"`
	ArtistInfo            *ArtistInfo            `xml:"artistInfo"            json:"artistInfo,omitempty"`
	ArtistInfoTwo         *ArtistInfo            `xml:"artistInfo2"           json:"artistInfo2,omitempty"`
	AlbumInfo             *AlbumInfo             `xml:"albumInfo"             json:"albumInfo,omitempty"`
	Genres                *Genres                `xml:"genres"                json:"genres,omitempty"`
	PlayQueue             *PlayQueue             `xml:"playQueue"             json:"playQueue,omitempty"`
	JukeboxStatus         *JukeboxStatus         `xml:"jukeboxStatus"         json:"jukeboxStatus,omitempty"`
	JukeboxPlaylist       *JukeboxPlaylist       `xml:"jukeboxPlaylist"       json:"jukeboxPlaylist,omitempty"`
	Podcasts              *Podcasts              `xml:"podcasts"              json:"podcasts,omitempty"`
	NewestPodcasts        *NewestPodcasts        `xml:"newestPodcasts"        json:"newestPodcasts,omitempty"`
	Bookmarks             *Bookmarks             `xml:"bookmarks"             json:"bookmarks,omitempty"`
	Starred               *Starred               `xml:"starred"               json:"starred,omitempty"`
	StarredTwo            *StarredTwo            `xml:"starred2"              json:"starred2,omitempty"`
	TopSongs              *TopSongs              `xml:"topSongs"              json:"topSongs,omitempty"`
	SimilarSongs          *SimilarSongs          `xml:"similarSongs"          json:"similarSongs,omitempty"`
	SimilarSongsTwo       *SimilarSongsTwo       `xml:"similarSongs2"         json:"similarSongs2,omitempty"`
	InternetRadioStations *InternetRadioStations `xml:"internetRadioStations" json:"internetRadioStations,omitempty"`
	Lyrics                *Lyrics                `xml:"lyrics"                json:"lyrics,omitempty"`
	LyricsList            *LyricsList            `xml:"lyricsList"            json:"lyricsList,omitempty"`
}

func NewResponse() *Response {
	return &Response{
		Status:        "ok",
		XMLNS:         xmlns,
		Version:       apiVersion,
		Type:          gonic.Name,
		ServerVersion: gonic.Version,
		OpenSubsonic:  true,
	}
}

// Error represents a typed error
//
//	0  a generic error
//
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
	r := NewResponse()
	r.Status = "failed"
	r.Error = &Error{
		Code:    code,
		Message: fmt.Sprintf(message, a...),
	}
	return r
}

type Albums struct {
	List []*Album `xml:"album" json:"album"`
}

type ArtistRef struct {
	ID   *specid.ID `xml:"id,attr" json:"id"`
	Name string     `xml:"name,attr" json:"name"`
}

type GenreRef struct {
	Name string `xml:"name,attr" json:"name"`
}

// https://opensubsonic.netlify.app/docs/responses/albumid3/
type Album struct {
	ID      *specid.ID `xml:"id,attr,omitempty"       json:"id"`
	Created time.Time  `xml:"created,attr,omitempty"  json:"created,omitempty"`

	// legacy or single tag mode
	ArtistID *specid.ID `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	Artist   string     `xml:"artist,attr"             json:"artist"`

	Artists       []*ArtistRef `xml:"artists"           json:"artists"`
	DisplayArtist string       `xml:"diplayArtist,attr" json:"displayArtist"`

	// folder stuff
	Title    string     `xml:"title,attr,omitempty"  json:"title"`
	Album    string     `xml:"album,attr,omitempty"  json:"album"`
	ParentID *specid.ID `xml:"parent,attr,omitempty" json:"parent,omitempty"`
	IsDir    bool       `xml:"isDir,attr,omitempty"  json:"isDir,omitempty"`
	CoverID  *specid.ID `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`

	Name       string        `xml:"name,attr"              json:"name"`
	TrackCount int           `xml:"songCount,attr"         json:"songCount"`
	Duration   int           `xml:"duration,attr"          json:"duration"`
	PlayCount  int           `xml:"playCount,attr"          json:"playCount"`
	Genre      string        `xml:"genre,attr,omitempty"   json:"genre,omitempty"`
	Genres     []*GenreRef   `xml:"genres,omitempty"       json:"genres,omitempty"`
	Year       int           `xml:"year,attr,omitempty"    json:"year,omitempty"`
	Tracks     []*TrackChild `xml:"song,omitempty"         json:"song,omitempty"`

	IsCompilation bool     `xml:"isCompilation" json:"isCompilation"`
	ReleaseTypes  []string `xml:"releaseTypes" json:"releaseTypes"`

	// star / rating
	Starred       *time.Time `xml:"starred,attr,omitempty"         json:"starred,omitempty"`
	UserRating    int        `xml:"userRating,attr,omitempty"      json:"userRating,omitempty"`
	AverageRating string     `xml:"averageRating,attr,omitempty"   json:"averageRating,omitempty"`
}

type RandomTracks struct {
	List []*TrackChild `xml:"song" json:"song"`
}

type TracksByGenre struct {
	List []*TrackChild `xml:"song" json:"song"`
}

type TranscodeMeta struct {
	TranscodedContentType string `xml:"transcodedContentType,attr,omitempty" json:"transcodedContentType,omitempty"`
	TranscodedSuffix      string `xml:"transcodedSuffix,attr,omitempty"      json:"transcodedSuffix,omitempty"`
}

type ReplayGain struct {
	TrackGain float32 `xml:"trackGain,attr" json:"trackGain"`
	TrackPeak float32 `xml:"trackPeak,attr" json:"trackPeak"`
	AlbumGain float32 `xml:"albumGain,attr" json:"albumGain"`
	AlbumPeak float32 `xml:"albumPeak,attr" json:"albumPeak"`
}

// https://opensubsonic.netlify.app/docs/responses/child/
type TrackChild struct {
	ID      *specid.ID `xml:"id,attr,omitempty"          json:"id,omitempty"`
	Album   string     `xml:"album,attr,omitempty"       json:"album,omitempty"`
	AlbumID *specid.ID `xml:"albumId,attr,omitempty"     json:"albumId,omitempty"`

	// legacy or single tag mode
	Artist   string     `xml:"artist,attr"             json:"artist"`
	ArtistID *specid.ID `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`

	Artists       []*ArtistRef `xml:"artists"           json:"artists"`
	DisplayArtist string       `xml:"diplayArtist,attr" json:"displayArtist"`

	AlbumArtists       []*ArtistRef `xml:"albumArtists"           json:"albumArtists"`
	AlbumDisplayArtist string       `xml:"diplayAlbumArtist,attr" json:"displayAlbumArtist"`

	Bitrate     int         `xml:"bitRate,attr,omitempty"     json:"bitRate,omitempty"`
	ContentType string      `xml:"contentType,attr,omitempty" json:"contentType,omitempty"`
	CoverID     *specid.ID  `xml:"coverArt,attr,omitempty"    json:"coverArt,omitempty"`
	CreatedAt   time.Time   `xml:"created,attr,omitempty"     json:"created,omitempty"`
	Duration    int         `xml:"duration,attr,omitempty"    json:"duration,omitempty"`
	Genre       string      `xml:"genre,attr,omitempty"       json:"genre,omitempty"`
	Genres      []*GenreRef `xml:"genres,omitempty"           json:"genres,omitempty"`
	IsDir       bool        `xml:"isDir,attr"                 json:"isDir"`
	IsVideo     bool        `xml:"isVideo,attr"               json:"isVideo"`
	ParentID    *specid.ID  `xml:"parent,attr,omitempty"      json:"parent,omitempty"`
	Path        string      `xml:"path,attr,omitempty"        json:"path,omitempty"`
	Size        int         `xml:"size,attr,omitempty"        json:"size,omitempty"`
	Suffix      string      `xml:"suffix,attr,omitempty"      json:"suffix,omitempty"`
	Title       string      `xml:"title,attr"                 json:"title"`
	TrackNumber int         `xml:"track,attr,omitempty"       json:"track,omitempty"`
	DiscNumber  int         `xml:"discNumber,attr,omitempty"  json:"discNumber,omitempty"`
	Type        string      `xml:"type,attr,omitempty"        json:"type,omitempty"`
	Year        int         `xml:"year,attr,omitempty"        json:"year,omitempty"`

	MusicBrainzID string `xml:"musicBrainzId,attr"        json:"musicBrainzId"`

	// star / rating
	Starred       *time.Time `xml:"starred,attr,omitempty"         json:"starred,omitempty"`
	UserRating    int        `xml:"userRating,attr,omitempty"      json:"userRating,omitempty"`
	AverageRating string     `xml:"averageRating,attr,omitempty"   json:"averageRating,omitempty"`

	ReplayGain *ReplayGain `xml:"replayGain" json:"replayGain"`

	TranscodeMeta
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
	// star / rating
	Starred       *time.Time `xml:"starred,attr,omitempty"       json:"starred,omitempty"`
	UserRating    int        `xml:"userRating,attr,omitempty"    json:"userRating,omitempty"`
	AverageRating string     `xml:"averageRating,attr,omitempty" json:"averageRating,omitempty"`
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
	ID            *specid.ID    `xml:"id,attr,omitempty"              json:"id"`
	ParentID      *specid.ID    `xml:"parent,attr,omitempty"          json:"parent,omitempty"`
	Name          string        `xml:"name,attr,omitempty"            json:"name"`
	Starred       *time.Time    `xml:"starred,attr,omitempty"         json:"starred,omitempty"`
	UserRating    int           `xml:"userRating,attr,omitempty"      json:"userRating,omitempty"`
	AverageRating string        `xml:"averageRating,attr,omitempty"   json:"averageRating,omitempty"`
	Children      []*TrackChild `xml:"child,omitempty"                json:"child,omitempty"`
}

type MusicFolders struct {
	List []*MusicFolder `xml:"musicFolder" json:"musicFolder"`
}

type MusicFolder struct {
	ID   int    `xml:"id,attr"             json:"id"`
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
	ID        string        `xml:"id,attr"         json:"id"`
	Name      string        `xml:"name,attr"       json:"name"`
	Comment   string        `xml:"comment,attr"    json:"comment"`
	Owner     string        `xml:"owner,attr"      json:"owner"`
	SongCount int           `xml:"songCount,attr"  json:"songCount"`
	Created   time.Time     `xml:"created,attr"    json:"created"`
	Changed   time.Time     `xml:"changed,attr"    json:"changed"`
	Duration  int           `xml:"duration,attr"   json:"duration"`
	Public    bool          `xml:"public,attr"     json:"public,omitempty"`
	List      []*TrackChild `xml:"entry,omitempty" json:"entry,omitempty"`
}

type ArtistInfo struct {
	Biography      string    `xml:"biography"               json:"biography"`
	MusicBrainzID  string    `xml:"musicBrainzId"           json:"musicBrainzId"`
	LastFMURL      string    `xml:"lastFmUrl"               json:"lastFmUrl"`
	SmallImageURL  string    `xml:"smallImageUrl"           json:"smallImageUrl"`
	MediumImageURL string    `xml:"mediumImageUrl"          json:"mediumImageUrl"`
	LargeImageURL  string    `xml:"largeImageUrl"           json:"largeImageUrl"`
	ArtistImageURL string    `xml:"artistImageUrl"          json:"artistImageUrl"` // not sure where this comes from but other clients seem to expect it
	Similar        []*Artist `xml:"similarArtist,omitempty" json:"similarArtist,omitempty"`
}

type AlbumInfo struct {
	Notes         string `xml:"notes"         json:"notes"`
	MusicBrainzID string `xml:"musicBrainzId" json:"musicBrainzId"`
	LastFMURL     string `xml:"lastFmUrl"     json:"lastFmUrl"`
}

type Genres struct {
	List []*Genre `xml:"genre" json:"genre"`
}

type Genre struct {
	Name       string `xml:",chardata"       json:"value"`
	SongCount  int    `xml:"songCount,attr"  json:"songCount"`
	AlbumCount int    `xml:"albumCount,attr" json:"albumCount"`
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
	*JukeboxStatus
}

type Podcasts struct {
	List []*PodcastChannel `xml:"channel" json:"channel"`
}

type NewestPodcasts struct {
	List []*PodcastEpisode `xml:"episode" json:"episode"`
}

type PodcastChannel struct {
	ID               *specid.ID        `xml:"id,attr"               json:"id"`
	URL              string            `xml:"url,attr"              json:"url"`
	Title            string            `xml:"title,attr"            json:"title"`
	Description      string            `xml:"description,attr"      json:"description"`
	CoverArt         *specid.ID        `xml:"coverArt,attr"         json:"coverArt,omitempty"`
	OriginalImageURL string            `xml:"originalImageUrl,attr" json:"originalImageUrl,omitempty"`
	Status           string            `xml:"status,attr"           json:"status"`
	Episode          []*PodcastEpisode `xml:"episode"               json:"episode,omitempty"`
}

type PodcastEpisode struct {
	ID          *specid.ID `xml:"id,attr"                  json:"id"`
	StreamID    *specid.ID `xml:"streamId,attr"            json:"streamId"`
	ChannelID   *specid.ID `xml:"channelId,attr"           json:"channelId"`
	Title       string     `xml:"title,attr"               json:"title"`
	Description string     `xml:"description,attr"         json:"description"`
	PublishDate time.Time  `xml:"publishDate,attr"         json:"publishDate"`
	Status      string     `xml:"status,attr"              json:"status"`
	Parent      string     `xml:"parent,attr"              json:"parent"`
	IsDir       bool       `xml:"isDir,attr"               json:"isDir"`
	Year        int        `xml:"year,attr"                json:"year"`
	Genre       string     `xml:"genre,attr"               json:"genre"`
	CoverArt    *specid.ID `xml:"coverArt,attr"            json:"coverArt"`
	Size        int        `xml:"size,attr"                json:"size"`
	ContentType string     `xml:"contentType,attr"         json:"contentType"`
	Suffix      string     `xml:"suffix,attr"              json:"suffix"`
	Duration    int        `xml:"duration,attr"            json:"duration"`
	BitRate     int        `xml:"bitRate,attr"             json:"bitrate"`
	Path        string     `xml:"path,attr"                json:"path"`
	Album       string     `xml:"album,attr"               json:"album"`
	Artist      string     `xml:"artist,attr"              json:"artist"`
}

type Bookmarks struct {
	List []*Bookmark `xml:"bookmark" json:"bookmark"`
}

type Bookmark struct {
	Entry    *TrackChild `xml:"entry,omitempty" json:"entry,omitempty"`
	Username string      `xml:"username,attr"   json:"username"`
	Position int         `xml:"position,attr"   json:"position"`
	Comment  string      `xml:"comment,attr"    json:"comment"`
	Created  time.Time   `xml:"created,attr"    json:"created"`
	Changed  time.Time   `xml:"changed,attr"    json:"changed"`
}

type Starred struct {
	Artists []*Directory  `xml:"artist,omitempty" json:"artist,omitempty"`
	Albums  []*TrackChild `xml:"album,omitempty"  json:"album,omitempty"`
	Tracks  []*TrackChild `xml:"song,omitempty"   json:"song,omitempty"`
}

type StarredTwo struct {
	Artists []*Artist     `xml:"artist,omitempty" json:"artist,omitempty"`
	Albums  []*Album      `xml:"album,omitempty"  json:"album,omitempty"`
	Tracks  []*TrackChild `xml:"song,omitempty"   json:"song,omitempty"`
}

type TopSongs struct {
	Tracks []*TrackChild `xml:"song,omitempty" json:"song,omitempty"`
}

type SimilarSongs struct {
	Tracks []*TrackChild `xml:"song,omitempty" json:"song,omitempty"`
}

type SimilarSongsTwo struct {
	Tracks []*TrackChild `xml:"song,omitempty" json:"song,omitempty"`
}

type InternetRadioStations struct {
	List []*InternetRadioStation `xml:"internetRadioStation" json:"internetRadioStation,omitempty"`
}

type InternetRadioStation struct {
	ID          *specid.ID `xml:"id,attr"          json:"id"`
	Name        string     `xml:"name,attr"        json:"name"`
	StreamURL   string     `xml:"streamUrl,attr"   json:"streamUrl"`
	HomepageURL string     `xml:"homepageUrl,attr" json:"homepageUrl"`
}

type Lyrics struct {
	Value  string `xml:",chardata"             json:"value,omitempty"`
	Artist string `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	Title  string `xml:"title,attr,omitempty"  json:"title,omitempty"`
}

type Lyric struct {
	Start int64  `xml:"start,attr" json:"start"`
	Value string `xml:",chardata" json:"value"`
}

type LyricsList struct {
	StructuredLyrics []StructuredLyrics `xml:"structuredLyrics" json:"structuredLyrics"`
}

type StructuredLyrics struct {
	Lang          string  `xml:"lang,attr" json:"lang"` // ISO 639 (or und, xxx if unknown)
	Synced        bool    `xml:"synced,attr" json:"synced"`
	Lines         []Lyric `xml:"line" json:"line"`
	DisplayArtist string  `xml:"displayArtist,attr,omitempty" json:"displayArtist,omitempty"`
	DisplayTitle  string  `xml:"displayTitle,attr,omitempty" json:"displayTitle,omitempty"`
	Offset        int     `xml:"offset,attr,omitempty" json:"offset,omitempty"`
}

type OpenSubsonicExtension struct {
	Name     string `xml:"name,attr" json:"name"`
	Versions []int  `xml:"versions"  json:"versions"`
}

type OpenSubsonicExtensions []OpenSubsonicExtension

func formatRating(rating float64) string {
	if rating == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f", rating)
}

func formatExt(ext string) string {
	return strings.TrimPrefix(ext, ".")
}

func formatReleaseTypes(types string) []string {
	parts := strings.Split(types, ",")
	if len(parts) == 0 {
		return []string{}
	}
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) == 0 {
			continue
		}
		part = string(unicode.ToUpper([]rune(part)[0])) + string([]rune(part)[1:])
		if part == "Ep" {
			part = "EP"
		}
		parts[i] = part
	}
	return parts
}

var doublePuncExpr = regexp.MustCompile(`\.\s+\.\s+`)
var licenceExpr = regexp.MustCompile(`(?i)\buser-contributed text.*`)
var readMoreExpr = regexp.MustCompile(`(?i)\bread more on.*`)

var bluemondayPolicy = bluemonday.StrictPolicy() //nolint:gochecknoglobals

func CleanExternalText(text string) string {
	text = bluemondayPolicy.Sanitize(text)
	text = html.UnescapeString(text)
	text = licenceExpr.ReplaceAllString(text, "")
	text = readMoreExpr.ReplaceAllString(text, "")
	text = doublePuncExpr.ReplaceAllString(text, ". ")
	text = strings.ReplaceAll(text, " .", ".")
	text = strings.Join(strings.Fields(text), " ")
	text = strings.TrimSpace(text)
	return text
}
