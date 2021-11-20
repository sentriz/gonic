package mockfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/server/scanner"
	"go.senan.xyz/gonic/server/scanner/tags"
)

var ErrPathNotFound = errors.New("path not found")

type MockFS struct {
	t       testing.TB
	scanner *scanner.Scanner
	dir     string
	reader  *mreader
	db      *db.DB
}

func New(t testing.TB) *MockFS {
	return new(t, []string{""})
}

func NewWithDirs(t testing.TB, dirs []string) *MockFS {
	return new(t, dirs)
}

func new(t testing.TB, dirs []string) *MockFS {
	dbc, err := db.NewMock()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	if err := dbc.Migrate(db.MigrationContext{}); err != nil {
		t.Fatalf("migrate db db: %v", err)
	}
	dbc.LogMode(false)

	tmpDir := t.TempDir()

	var absDirs []string
	for _, dir := range dirs {
		absDirs = append(absDirs, filepath.Join(tmpDir, dir))
	}
	for _, absDir := range absDirs {
		if err := os.MkdirAll(absDir, os.ModePerm); err != nil {
			t.Fatalf("mk abs dir: %v", err)
		}
	}

	parser := &mreader{map[string]*Tags{}}
	scanner := scanner.New(absDirs, dbc, ";", parser)

	return &MockFS{
		t:       t,
		scanner: scanner,
		dir:     tmpDir,
		reader:  parser,
		db:      dbc,
	}
}

func (m *MockFS) DB() *db.DB     { return m.db }
func (m *MockFS) TmpDir() string { return m.dir }

func (m *MockFS) ScanAndClean() {
	if err := m.scanner.ScanAndClean(scanner.ScanOptions{}); err != nil {
		m.t.Fatalf("error scan and cleaning: %v", err)
	}
}

func (m *MockFS) ResetDates() {
	t := time.Date(2020, 0, 0, 0, 0, 0, 0, time.UTC)
	if err := m.db.Model(db.Album{}).Updates(db.Album{CreatedAt: t, UpdatedAt: t, ModifiedAt: t}).Error; err != nil {
		m.t.Fatalf("reset album times: %v", err)
	}
	if err := m.db.Model(db.Track{}).Updates(db.Track{CreatedAt: t, UpdatedAt: t}).Error; err != nil {
		m.t.Fatalf("reset track times: %v", err)
	}
}

func (m *MockFS) CleanUp() {
	if err := m.db.Close(); err != nil {
		m.t.Fatalf("close db: %v", err)
	}
}

func (m *MockFS) addItems(prefix string, covers bool) {
	p := func(format string, a ...interface{}) string {
		return filepath.Join(prefix, fmt.Sprintf(format, a...))
	}
	for ar := 0; ar < 3; ar++ {
		for al := 0; al < 3; al++ {
			for tr := 0; tr < 3; tr++ {
				m.AddTrack(p("artist-%d/album-%d/track-%d.flac", ar, al, tr))
				m.SetTags(p("artist-%d/album-%d/track-%d.flac", ar, al, tr), func(tags *Tags) {
					tags.RawArtist = fmt.Sprintf("artist-%d", ar)
					tags.RawAlbumArtist = fmt.Sprintf("artist-%d", ar)
					tags.RawAlbum = fmt.Sprintf("album-%d", al)
					tags.RawTitle = fmt.Sprintf("title-%d", tr)
				})
			}
			if covers {
				m.AddCover(p("artist-%d/album-%d/cover.png", ar, al))
			}
		}
	}
}

func (m *MockFS) AddItems()                              { m.addItems("", false) }
func (m *MockFS) AddItemsPrefix(prefix string)           { m.addItems(prefix, false) }
func (m *MockFS) AddItemsWithCovers()                    { m.addItems("", true) }
func (m *MockFS) AddItemsPrefixWithCovers(prefix string) { m.addItems(prefix, true) }

func (m *MockFS) RemoveAll(path string) {
	abspath := filepath.Join(m.dir, path)
	if err := os.RemoveAll(abspath); err != nil {
		m.t.Fatalf("remove all: %v", err)
	}
}

func (m *MockFS) LogItems() {
	m.t.Logf("\nitems")
	var dirs int
	err := filepath.Walk(m.dir, func(path string, info fs.FileInfo, err error) error {
		m.t.Logf("item %q", path)
		if info.IsDir() {
			dirs++
		}
		return nil
	})
	if err != nil {
		m.t.Fatalf("error logging items: %v", err)
	}
	m.t.Logf("total %d", dirs)
}

func (m *MockFS) LogAlbums() {
	var albums []*db.Album
	if err := m.db.Find(&albums).Error; err != nil {
		m.t.Fatalf("error logging items: %v", err)
	}

	m.t.Logf("\nalbums")
	for _, album := range albums {
		m.t.Logf("id %-3d root %-3s lr %-15s %-10s pid %-3d aid %-3d cov %-10s",
			album.ID, album.RootDir, album.LeftPath, album.RightPath, album.ParentID, album.TagArtistID, album.Cover)
	}
	m.t.Logf("total %d", len(albums))
}

func (m *MockFS) LogArtists() {
	var artists []*db.Artist
	if err := m.db.Find(&artists).Error; err != nil {
		m.t.Fatalf("error logging items: %v", err)
	}

	m.t.Logf("\nartists")
	for _, artist := range artists {
		m.t.Logf("id %-3d %-10s", artist.ID, artist.Name)
	}
	m.t.Logf("total %d", len(artists))
}

func (m *MockFS) LogTracks() {
	var tracks []*db.Track
	if err := m.db.Find(&tracks).Error; err != nil {
		m.t.Fatalf("error logging items: %v", err)
	}

	m.t.Logf("\ntracks")
	for _, track := range tracks {
		m.t.Logf("id %-3d aid %-3d filename %-10s tagtitle %-10s",
			track.ID, track.AlbumID, track.Filename, track.TagTitle)
	}
}

func (m *MockFS) LogTrackGenres() {
	var tgs []*db.TrackGenre
	if err := m.db.Find(&tgs).Error; err != nil {
		m.t.Fatalf("error logging items: %v", err)
	}

	m.t.Logf("\ntrack genres")
	for _, tg := range tgs {
		m.t.Logf("tid %-3d gid %-3d", tg.TrackID, tg.GenreID)
	}
}

func (m *MockFS) AddTrack(path string) {
	abspath := filepath.Join(m.dir, path)
	dir := filepath.Dir(abspath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		m.t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(abspath)
	if err != nil {
		m.t.Fatalf("create track: %v", err)
	}
	defer f.Close()
}

func (m *MockFS) AddCover(path string) {
	abspath := filepath.Join(m.dir, path)
	if err := os.MkdirAll(filepath.Dir(abspath), os.ModePerm); err != nil {
		m.t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(abspath)
	if err != nil {
		m.t.Fatalf("create cover: %v", err)
	}
	defer f.Close()
}

func (m *MockFS) SetTags(path string, cb func(*Tags)) {
	abspath := filepath.Join(m.dir, path)
	if err := os.Chtimes(abspath, time.Time{}, time.Now()); err != nil {
		m.t.Fatalf("touch track: %v", err)
	}
	if _, ok := m.reader.tags[abspath]; !ok {
		m.reader.tags[abspath] = &Tags{}
	}
	cb(m.reader.tags[abspath])
}

type mreader struct {
	tags map[string]*Tags
}

func (m *mreader) Read(abspath string) (tags.Parser, error) {
	parser, ok := m.tags[abspath]
	if !ok {
		return nil, ErrPathNotFound
	}
	return parser, nil
}

var _ tags.Reader = (*mreader)(nil)

type Tags struct {
	RawTitle       string
	RawArtist      string
	RawAlbum       string
	RawAlbumArtist string
	RawGenre       string
}

func (m *Tags) Title() string         { return m.RawTitle }
func (m *Tags) BrainzID() string      { return "" }
func (m *Tags) Artist() string        { return m.RawArtist }
func (m *Tags) Album() string         { return m.RawAlbum }
func (m *Tags) AlbumArtist() string   { return m.RawAlbumArtist }
func (m *Tags) AlbumBrainzID() string { return "" }
func (m *Tags) Genre() string         { return m.RawGenre }
func (m *Tags) TrackNumber() int      { return 1 }
func (m *Tags) DiscNumber() int       { return 1 }
func (m *Tags) Length() int           { return 100 }
func (m *Tags) Bitrate() int          { return 100 }
func (m *Tags) Year() int             { return 2021 }

func (m *Tags) SomeAlbum() string       { return m.Album() }
func (m *Tags) SomeArtist() string      { return m.Artist() }
func (m *Tags) SomeAlbumArtist() string { return m.AlbumArtist() }
func (m *Tags) SomeGenre() string       { return m.Genre() }

var _ tags.Parser = (*Tags)(nil)
