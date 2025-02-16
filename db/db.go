package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/mattn/go-sqlite3"

	"go.senan.xyz/gonic/sandbox"

	// TODO: remove this dep
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

func DefaultOptions() url.Values {
	return url.Values{
		// with this, multiple connections share a single data and schema cache.
		// see https://www.sqlite.org/sharedcache.html
		"cache": {"shared"},
		// with this, the db sleeps for a little while when locked. can prevent
		// a SQLITE_BUSY. see https://www.sqlite.org/c3ref/busy_timeout.html
		"_busy_timeout": {"30000"},
		"_journal_mode": {"WAL"},
		"_foreign_keys": {"true"},
	}
}

func mockOptions() url.Values {
	return url.Values{
		"_foreign_keys": {"true"},
	}
}

type DB struct {
	*gorm.DB
}

func New(path string, options url.Values) (*DB, error) {
	// https://github.com/mattn/go-sqlite3#connection-string
	url := url.URL{
		Scheme: "file",
		Opaque: path,
	}
	sandbox.ReadWriteCreatePath(path)
	sandbox.ReadWriteCreatePath(path + "-wal")
	sandbox.ReadWriteCreatePath(path + "-shm")
	sandbox.ReadWriteCreatePath(path + "-journal")
	url.RawQuery = options.Encode()
	db, err := gorm.Open("sqlite3", url.String())
	if err != nil {
		return nil, fmt.Errorf("with gorm: %w", err)
	}
	db.SetLogger(log.New(os.Stdout, "gorm ", 0))
	db.DB().SetMaxOpenConns(1)
	return &DB{DB: db}, nil
}

func NewMock() (*DB, error) {
	return New(":memory:", mockOptions())
}

func (db *DB) InsertBulkLeftMany(table string, head []string, left int, col []int) error {
	if len(col) == 0 {
		return nil
	}
	var rows []string
	var values []interface{}
	for _, c := range col {
		rows = append(rows, "(?, ?)")
		values = append(values, left, c)
	}
	q := fmt.Sprintf("INSERT OR IGNORE INTO %q (%s) VALUES %s",
		table,
		strings.Join(head, ", "),
		strings.Join(rows, ", "),
	)
	return db.Exec(q, values...).Error
}

type Stats struct {
	Folders, Albums, Artists, AlbumArtists, Tracks, InternetRadioStations, Podcasts uint
}

func (db *DB) Stats() (Stats, error) {
	var stats Stats
	db.Model(Album{}).Count(&stats.Folders)
	db.Model(AlbumArtist{}).Group("album_id").Count(&stats.Albums)
	db.Model(TrackArtist{}).Group("artist_id").Count(&stats.Artists)
	db.Model(AlbumArtist{}).Group("artist_id").Count(&stats.AlbumArtists)
	db.Model(Track{}).Count(&stats.Tracks)
	db.Model(InternetRadioStation{}).Count(&stats.InternetRadioStations)
	db.Model(Podcast{}).Count(&stats.Podcasts)
	return stats, nil
}

func (db *DB) GetUserByID(id int) *User {
	var user User
	err := db.
		Where("id=?", id).
		First(&user).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return &user
}

func (db *DB) GetUserByName(name string) *User {
	var user User
	err := db.
		Where("name=?", name).
		First(&user).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return &user
}

func (db *DB) Begin() *DB {
	return &DB{DB: db.DB.Begin()}
}

func (db *DB) Transaction(cb func(*DB) error) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		return cb(&DB{DB: tx})
	})
}

func (db *DB) TransactionChunked(data []int64, cb func(*DB, []int64) error) error {
	if len(data) == 0 {
		return nil
	}
	// https://sqlite.org/limits.html
	const size = 999
	return db.Transaction(func(tx *DB) error {
		for i := 0; i < len(data); i += size {
			end := i + size
			if end > len(data) {
				end = len(data)
			}
			if err := cb(tx, data[i:end]); err != nil {
				return err
			}
		}
		return nil
	})
}

type SettingKey string

const (
	LastFMAPIKey SettingKey = "lastfm_api_key" //nolint:gosec
	LastFMSecret SettingKey = "lastfm_secret"
	LastScanTime SettingKey = "last_scan_time"
)

func (db *DB) GetSetting(key SettingKey) (string, error) {
	var setting Setting
	if err := db.Where("key=?", key).First(&setting).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	return setting.Value, nil
}

func (db *DB) SetSetting(key SettingKey, value string) error {
	return db.
		Where("key=?", key).
		Assign(Setting{Key: key, Value: value}).
		FirstOrCreate(&Setting{}).
		Error
}

type Artist struct {
	ID              int      `gorm:"primary_key"`
	Name            string   `gorm:"not null; unique_index"`
	NameUDec        string   `sql:"default: null"`
	Albums          []*Album `gorm:"many2many:album_artists"`
	AlbumCount      int      `sql:"-"`
	Appearances     []*Album `gorm:"many2many:artist_appearances"`
	AppearanceCount int      `sql:"-"`
	ArtistStar      *ArtistStar
	ArtistRating    *ArtistRating
	AverageRating   float64     `sql:"default: null"`
	Info            *ArtistInfo `gorm:"foreignkey:id"`
}

func (a *Artist) SID() *specid.ID {
	return &specid.ID{Type: specid.Artist, Value: a.ID}
}

func (a *Artist) IndexName() string {
	if len(a.NameUDec) > 0 {
		return a.NameUDec
	}
	return a.Name
}

type Genre struct {
	ID         int    `gorm:"primary_key"`
	Name       string `gorm:"not null; unique_index"`
	AlbumCount int    `sql:"-"`
	TrackCount int    `sql:"-"`
}

// AudioFile is used to avoid some duplication in handlers_raw.go
// between Track and Podcast
type AudioFile interface {
	Ext() string
	MIME() string
	AudioFilename() string
	AudioBitrate() int
	AudioLength() int
}

type Track struct {
	ID             int `gorm:"primary_key"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Filename       string `gorm:"not null; unique_index:idx_folder_filename" sql:"default: null"`
	FilenameUDec   string `sql:"default: null"`
	Album          *Album
	AlbumID        int       `gorm:"not null; unique_index:idx_folder_filename" sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	Artists        []*Artist `gorm:"many2many:track_artists"`
	Genres         []*Genre  `gorm:"many2many:track_genres"`
	Size           int       `sql:"default: null"`
	Length         int       `sql:"default: null"`
	Bitrate        int       `sql:"default: null"`
	TagTitle       string    `sql:"default: null"`
	TagTitleUDec   string    `sql:"default: null"`
	TagTrackArtist string    `sql:"default: null"`
	TagTrackNumber int       `sql:"default: null"`
	TagDiscNumber  int       `sql:"default: null"`
	TagBrainzID    string    `sql:"default: null"`

	ReplayGainTrackGain float32
	ReplayGainTrackPeak float32
	ReplayGainAlbumGain float32
	ReplayGainAlbumPeak float32

	TrackStar     *TrackStar
	TrackRating   *TrackRating
	AverageRating float64 `sql:"default: null"`
}

func (t *Track) AudioLength() int  { return t.Length }
func (t *Track) AudioBitrate() int { return t.Bitrate }

func (t *Track) SID() *specid.ID {
	return &specid.ID{Type: specid.Track, Value: t.ID}
}

func (t *Track) AlbumSID() *specid.ID {
	return &specid.ID{Type: specid.Album, Value: t.AlbumID}
}

func (t *Track) Ext() string {
	return filepath.Ext(t.Filename)
}

func (t *Track) AudioFilename() string {
	return t.Filename
}

func (t *Track) MIME() string {
	return mime.TypeByExtension(filepath.Ext(t.Filename))
}

func (t *Track) AbsPath() string {
	if t.Album == nil {
		return ""
	}
	return filepath.Join(
		t.Album.RootDir,
		t.Album.LeftPath,
		t.Album.RightPath,
		t.Filename,
	)
}

func (t *Track) RelPath() string {
	if t.Album == nil {
		return ""
	}
	return filepath.Join(
		t.Album.LeftPath,
		t.Album.RightPath,
		t.Filename,
	)
}

type User struct {
	ID                int `gorm:"primary_key"`
	CreatedAt         time.Time
	Name              string `gorm:"not null; unique_index" sql:"default: null"`
	Password          string `gorm:"not null" sql:"default: null"`
	LastFMSession     string `sql:"default: null"`
	ListenBrainzURL   string `sql:"default: null"`
	ListenBrainzToken string `sql:"default: null"`
	IsAdmin           bool   `sql:"default: null"`
	Avatar            []byte `sql:"default: null"`
}

type Setting struct {
	Key   SettingKey `gorm:"not null; primary_key; auto_increment:false" sql:"default: null"`
	Value string     `sql:"default: null"`
}

type Play struct {
	ID      int `gorm:"primary_key"`
	User    *User
	UserID  int `gorm:"not null; index" sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	Album   *Album
	AlbumID int       `gorm:"not null; index" sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	Time    time.Time `sql:"default: null"`
	Count   int
	Length  int
}

type Album struct {
	ID             int `gorm:"primary_key"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ModifiedAt     time.Time
	LeftPath       string `gorm:"unique_index:idx_album_abs_path"`
	RightPath      string `gorm:"not null; unique_index:idx_album_abs_path" sql:"default: null"`
	RightPathUDec  string `sql:"default: null"`
	Parent         *Album
	ParentID       int       `sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	RootDir        string    `gorm:"unique_index:idx_album_abs_path" sql:"default: null"`
	Genres         []*Genre  `gorm:"many2many:album_genres"`
	Cover          string    `sql:"default: null"`
	Artists        []*Artist `gorm:"many2many:album_artists"`
	TagTitle       string    `sql:"default: null"`
	TagAlbumArtist string    // display purposes only
	TagTitleUDec   string    `sql:"default: null"`
	TagBrainzID    string    `sql:"default: null"`
	TagYear        int       `sql:"default: null"`
	Tracks         []*Track
	ChildCount     int `sql:"-"`
	Duration       int `sql:"-"`
	AlbumStar      *AlbumStar
	AlbumRating    *AlbumRating
	AverageRating  float64 `sql:"default: null"`
	Play           *Play
}

func (a *Album) SID() *specid.ID {
	return &specid.ID{Type: specid.Album, Value: a.ID}
}

func (a *Album) ParentSID() *specid.ID {
	return &specid.ID{Type: specid.Album, Value: a.ParentID}
}

func (a *Album) IndexRightPath() string {
	if len(a.RightPathUDec) > 0 {
		return a.RightPathUDec
	}
	return a.RightPath
}

type PlayQueue struct {
	ID        int `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
	User      *User
	UserID    int `sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	Current   string
	Position  int
	ChangedBy string
	Items     string
}

func (p *PlayQueue) CurrentSID() *specid.ID {
	id, _ := specid.New(p.Current)
	return &id
}

func (p *PlayQueue) GetItems() []specid.ID {
	return splitIDs(p.Items, ",")
}

func (p *PlayQueue) SetItems(items []specid.ID) {
	p.Items = join(items, ",")
}

type TranscodePreference struct {
	UserID  int    `gorm:"not null; unique_index:idx_user_id_client" sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	Client  string `gorm:"not null; unique_index:idx_user_id_client" sql:"default: null"`
	Profile string `gorm:"not null" sql:"default: null"`
}

type AlbumArtist struct {
	AlbumID  int `gorm:"not null; unique_index:idx_album_id_artist_id" sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	ArtistID int `gorm:"not null; unique_index:idx_album_id_artist_id" sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
}

type TrackArtist struct {
	TrackID  int `gorm:"not null; unique_index:idx_track_id_artist_id" sql:"default: null; type:int REFERENCES tracks(id) ON DELETE CASCADE"`
	ArtistID int `gorm:"not null; unique_index:idx_track_id_artist_id" sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
}

type ArtistAppearances struct {
	ArtistID int `gorm:"not null; unique_index:idx_artist_id_album_id" sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
	AlbumID  int `gorm:"not null; unique_index:idx_artist_id_album_id" sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
}

type TrackGenre struct {
	TrackID int `gorm:"not null; unique_index:idx_track_id_genre_id" sql:"default: null; type:int REFERENCES tracks(id) ON DELETE CASCADE"`
	GenreID int `gorm:"not null; unique_index:idx_track_id_genre_id" sql:"default: null; type:int REFERENCES genres(id) ON DELETE CASCADE"`
}

type AlbumGenre struct {
	AlbumID int `gorm:"not null; unique_index:idx_album_id_genre_id" sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	GenreID int `gorm:"not null; unique_index:idx_album_id_genre_id" sql:"default: null; type:int REFERENCES genres(id) ON DELETE CASCADE"`
}

type AlbumStar struct {
	UserID   int `gorm:"primary_key; not null" sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	AlbumID  int `gorm:"primary_key; not null" sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	StarDate time.Time
}

type AlbumRating struct {
	UserID  int `gorm:"primary_key; not null" sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	AlbumID int `gorm:"primary_key; not null" sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	Rating  int `gorm:"not null; check:(rating >= 1 AND rating <= 5)"`
}

type ArtistStar struct {
	UserID   int `gorm:"primary_key; not null" sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	ArtistID int `gorm:"primary_key; not null" sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
	StarDate time.Time
}

type ArtistRating struct {
	UserID   int `gorm:"primary_key; not null" sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	ArtistID int `gorm:"primary_key; not null" sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
	Rating   int `gorm:"not null; check:(rating >= 1 AND rating <= 5)"`
}

type TrackStar struct {
	UserID   int `gorm:"primary_key; not null" sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	TrackID  int `gorm:"primary_key; not null" sql:"default: null; type:int REFERENCES tracks(id) ON DELETE CASCADE"`
	StarDate time.Time
}

type TrackRating struct {
	UserID  int `gorm:"primary_key; not null" sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	TrackID int `gorm:"primary_key; not null" sql:"default: null; type:int REFERENCES tracks(id) ON DELETE CASCADE"`
	Rating  int `gorm:"not null; check:(rating >= 1 AND rating <= 5)"`
}

type PodcastAutoDownload string

const (
	PodcastAutoDownloadLatest PodcastAutoDownload = "latest"
	PodcastAutoDownloadNone   PodcastAutoDownload = "none"
)

type Podcast struct {
	ID           int `gorm:"primary_key"`
	UpdatedAt    time.Time
	ModifiedAt   time.Time
	URL          string
	Title        string
	Description  string
	ImageURL     string
	Image        string
	Error        string
	Episodes     []*PodcastEpisode
	AutoDownload PodcastAutoDownload
	RootDir      string
}

func (p *Podcast) SID() *specid.ID {
	return &specid.ID{Type: specid.Podcast, Value: p.ID}
}

type PodcastEpisodeStatus string

const (
	PodcastEpisodeStatusDownloading PodcastEpisodeStatus = "downloading"
	PodcastEpisodeStatusSkipped     PodcastEpisodeStatus = "skipped"
	PodcastEpisodeStatusDeleted     PodcastEpisodeStatus = "deleted"
	PodcastEpisodeStatusCompleted   PodcastEpisodeStatus = "completed"
	PodcastEpisodeStatusError       PodcastEpisodeStatus = "error"
)

type PodcastEpisode struct {
	ID          int `gorm:"primary_key"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ModifiedAt  time.Time
	PodcastID   int `gorm:"not null" sql:"default: null; type:int REFERENCES podcasts(id) ON DELETE CASCADE"`
	Title       string
	Description string
	PublishDate *time.Time
	AudioURL    string
	Bitrate     int
	Length      int
	Size        int
	Filename    string
	Status      PodcastEpisodeStatus
	Error       string
	Podcast     *Podcast
}

func (pe *PodcastEpisode) AudioLength() int  { return pe.Length }
func (pe *PodcastEpisode) AudioBitrate() int { return pe.Bitrate }

func (pe *PodcastEpisode) SID() *specid.ID {
	return &specid.ID{Type: specid.PodcastEpisode, Value: pe.ID}
}

func (pe *PodcastEpisode) PodcastSID() *specid.ID {
	return &specid.ID{Type: specid.Podcast, Value: pe.PodcastID}
}

func (pe *PodcastEpisode) AudioFilename() string {
	return pe.Filename
}

func (pe *PodcastEpisode) Ext() string {
	return filepath.Ext(pe.Filename)
}

func (pe *PodcastEpisode) MIME() string {
	return mime.TypeByExtension(filepath.Ext(pe.Filename))
}

func (pe *PodcastEpisode) AbsPath() string {
	if pe.Podcast == nil || pe.Podcast.RootDir == "" {
		return ""
	}
	return filepath.Join(pe.Podcast.RootDir, pe.Filename)
}

type Bookmark struct {
	ID          int `gorm:"primary_key"`
	User        *User
	UserID      int `sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	Position    int
	Comment     string
	EntryIDType string
	EntryID     int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type InternetRadioStation struct {
	ID          int `gorm:"primary_key"`
	StreamURL   string
	Name        string
	HomepageURL string
}

func (ir *InternetRadioStation) SID() *specid.ID {
	return &specid.ID{Type: specid.InternetRadioStation, Value: ir.ID}
}

func (ir *InternetRadioStation) AbsPath() string {
	return ir.StreamURL
}

type ArtistInfo struct {
	ID             int `gorm:"primary_key" sql:"type:int REFERENCES artists(id) ON DELETE CASCADE"`
	CreatedAt      time.Time
	UpdatedAt      time.Time `gorm:"index"`
	Biography      string
	MusicBrainzID  string
	LastFMURL      string
	ImageURL       string
	SimilarArtists string
	TopTracks      string
}

func (p *ArtistInfo) GetSimilarArtists() []string      { return strings.Split(p.SimilarArtists, ";") }
func (p *ArtistInfo) SetSimilarArtists(items []string) { p.SimilarArtists = strings.Join(items, ";") }

func (p *ArtistInfo) GetTopTracks() []string      { return strings.Split(p.TopTracks, ";") }
func (p *ArtistInfo) SetTopTracks(items []string) { p.TopTracks = strings.Join(items, ";") }

type AlbumInfo struct {
	ID            int `gorm:"primary_key" sql:"type:int REFERENCES albums(id) ON DELETE CASCADE"`
	CreatedAt     time.Time
	UpdatedAt     time.Time `gorm:"index"`
	Notes         string
	MusicBrainzID string
	LastFMURL     string
}

func splitIDs(in, sep string) []specid.ID {
	if in == "" {
		return []specid.ID{}
	}
	parts := strings.Split(in, sep)
	ret := make([]specid.ID, 0, len(parts))
	for _, p := range parts {
		id, _ := specid.New(p)
		ret = append(ret, id)
	}
	return ret
}

func join[T fmt.Stringer](in []T, sep string) string {
	if in == nil {
		return ""
	}
	strs := make([]string, 0, len(in))
	for _, id := range in {
		strs = append(strs, id.String())
	}
	return strings.Join(strs, sep)
}

func Dump(ctx context.Context, db *gorm.DB, to string) error {
	dest, err := New(to, url.Values{})
	if err != nil {
		return fmt.Errorf("create dest db: %w", err)
	}
	defer dest.Close()

	connSrc, err := db.DB().Conn(ctx)
	if err != nil {
		return fmt.Errorf("getting src raw conn: %w", err)
	}
	defer connSrc.Close()

	connDest, err := dest.DB.DB().Conn(ctx)
	if err != nil {
		return fmt.Errorf("getting dest raw conn: %w", err)
	}
	defer connDest.Close()

	err = connDest.Raw(func(connDest interface{}) error {
		return connSrc.Raw(func(connSrc interface{}) error {
			connDestq := connDest.(*sqlite3.SQLiteConn)
			connSrcq := connSrc.(*sqlite3.SQLiteConn)
			bk, err := connDestq.Backup("main", connSrcq, "main")
			if err != nil {
				return fmt.Errorf("create backup db: %w", err)
			}
			for done, _ := bk.Step(-1); !done; { //nolint: revive
			}
			if err := bk.Finish(); err != nil {
				return fmt.Errorf("finishing dump: %w", err)
			}
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("backing up: %w", err)
	}

	return nil
}
