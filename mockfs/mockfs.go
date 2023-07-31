package mockfs

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scanner"
	"go.senan.xyz/gonic/scanner/tags"
)

var ErrPathNotFound = errors.New("path not found")

type MockFS struct {
	t         testing.TB
	scanner   *scanner.Scanner
	dir       string
	tagReader *tagReader
	db        *db.DB
}

func New(t testing.TB) *MockFS                        { return newMockFS(t, []string{""}, "") }
func NewWithDirs(t testing.TB, dirs []string) *MockFS { return newMockFS(t, dirs, "") }
func NewWithExcludePattern(t testing.TB, excludePattern string) *MockFS {
	return newMockFS(t, []string{""}, excludePattern)
}

func newMockFS(t testing.TB, dirs []string, excludePattern string) *MockFS {
	dbc, err := db.NewMock()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	t.Cleanup(func() {
		if err := dbc.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

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

	tagReader := &tagReader{paths: map[string]*tagReaderResult{}}
	scanner := scanner.New(absDirs, dbc, ";", tagReader, excludePattern)

	return &MockFS{
		t:         t,
		scanner:   scanner,
		dir:       tmpDir,
		tagReader: tagReader,
		db:        dbc,
	}
}

func (m *MockFS) DB() *db.DB     { return m.db }
func (m *MockFS) TmpDir() string { return m.dir }

func (m *MockFS) ScanAndClean() *scanner.Context {
	ctx, err := m.scanner.ScanAndClean(scanner.ScanOptions{})
	if err != nil {
		m.t.Fatalf("error scan and cleaning: %v", err)
	}
	return ctx
}

func (m *MockFS) ScanAndCleanErr() (*scanner.Context, error) {
	return m.scanner.ScanAndClean(scanner.ScanOptions{})
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

func (m *MockFS) AddItems()                              { m.addItems("", false) }
func (m *MockFS) AddItemsPrefix(prefix string)           { m.addItems(prefix, false) }
func (m *MockFS) AddItemsWithCovers()                    { m.addItems("", true) }
func (m *MockFS) AddItemsPrefixWithCovers(prefix string) { m.addItems(prefix, true) }

func (m *MockFS) addItems(prefix string, covers bool) {
	p := func(format string, a ...interface{}) string {
		return filepath.Join(prefix, fmt.Sprintf(format, a...))
	}
	for ar := 0; ar < 3; ar++ {
		for al := 0; al < 3; al++ {
			for tr := 0; tr < 3; tr++ {
				m.AddTrack(p("artist-%d/album-%d/track-%d.flac", ar, al, tr))
				m.SetTags(p("artist-%d/album-%d/track-%d.flac", ar, al, tr), func(tags *Tags) error {
					tags.RawArtist = fmt.Sprintf("artist-%d", ar)
					tags.RawAlbumArtist = fmt.Sprintf("artist-%d", ar)
					tags.RawAlbum = fmt.Sprintf("album-%d", al)
					tags.RawTitle = fmt.Sprintf("title-%d", tr)
					return nil
				})
			}
			if covers {
				m.AddCover(p("artist-%d/album-%d/cover.png", ar, al))
			}
		}
	}
}

func (m *MockFS) NumTracks() int {
	return len(m.tagReader.paths)
}

func (m *MockFS) RemoveAll(path string) {
	abspath := filepath.Join(m.dir, path)
	if err := os.RemoveAll(abspath); err != nil {
		m.t.Fatalf("remove all: %v", err)
	}
}

func (m *MockFS) Symlink(src, dest string) {
	if err := os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
		m.t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(src, dest); err != nil {
		m.t.Fatalf("symlink: %v", err)
	}
	src = filepath.Clean(src)
	dest = filepath.Clean(dest)
	for k, v := range m.tagReader.paths {
		m.tagReader.paths[strings.Replace(k, src, dest, 1)] = v
	}
}

func (m *MockFS) SetRealAudio(path string, length int, audioPath string) {
	abspath := filepath.Join(m.dir, path)
	if err := os.Remove(abspath); err != nil {
		m.t.Fatalf("remove all: %v", err)
	}
	wd, _ := os.Getwd()
	if err := os.Symlink(filepath.Join(wd, audioPath), abspath); err != nil {
		m.t.Fatalf("symlink: %v", err)
	}
	m.SetTags(path, func(tags *Tags) error {
		tags.RawLength = length
		tags.RawBitrate = 0
		return nil
	})
}

func (m *MockFS) LogItems() {
	m.t.Logf("\nitems")
	var items int
	err := filepath.WalkDir(m.dir, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch info.Type() {
		case os.ModeSymlink:
			m.t.Logf("item %q [sym]", path)
		default:
			m.t.Logf("item %q", path)
		}
		items++
		return nil
	})
	if err != nil {
		m.t.Fatalf("error logging items: %v", err)
	}
	m.t.Logf("total %d", items)
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
	m.t.Logf("total %d", len(tracks))
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
	m.t.Logf("total %d", len(tgs))
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

func (m *MockFS) SetTags(path string, cb func(*Tags) error) {
	abspath := filepath.Join(m.dir, path)
	if err := os.Chtimes(abspath, time.Time{}, time.Now()); err != nil {
		m.t.Fatalf("touch track: %v", err)
	}
	r := m.tagReader
	if _, ok := r.paths[abspath]; !ok {
		r.paths[abspath] = &tagReaderResult{tags: &Tags{}}
	}
	if err := cb(r.paths[abspath].tags); err != nil {
		r.paths[abspath].err = err
	}
}

func (m *MockFS) DumpDB(suffix ...string) {
	var p []string
	p = append(p,
		"gonic", "dump",
		strings.ReplaceAll(m.t.Name(), string(filepath.Separator), "-"),
		fmt.Sprint(time.Now().UnixNano()),
	)
	p = append(p, suffix...)

	destPath := filepath.Join(os.TempDir(), strings.Join(p, "-"))
	dest, err := db.New(destPath, url.Values{})
	if err != nil {
		m.t.Fatalf("create dest db: %v", err)
	}
	defer dest.Close()

	connSrc, err := m.db.DB.DB().Conn(context.Background())
	if err != nil {
		m.t.Fatalf("getting src raw conn: %v", err)
	}
	defer connSrc.Close()
	connDest, err := dest.DB.DB().Conn(context.Background())
	if err != nil {
		m.t.Fatalf("getting dest raw conn: %v", err)
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
			for done, _ := bk.Step(-1); !done; {
				m.t.Logf("dumping db...")
			}
			if err := bk.Finish(); err != nil {
				return fmt.Errorf("finishing dump: %w", err)
			}
			return nil
		})
	})
	if err != nil {
		m.t.Fatalf("backing up: %v", err)
	}
}

type tagReaderResult struct {
	tags *Tags
	err  error
}

type tagReader struct {
	paths map[string]*tagReaderResult
}

func (m *tagReader) Read(abspath string) (tags.Parser, error) {
	p, ok := m.paths[abspath]
	if !ok {
		return nil, ErrPathNotFound
	}
	return p.tags, p.err
}

var _ tags.Reader = (*tagReader)(nil)

type Tags struct {
	RawTitle        string
	RawArtist       string
	RawAlbum        string
	RawAlbumArtist  string
	RawAlbumArtists []string
	RawGenre        string

	RawBitrate int
	RawLength  int
}

func (m *Tags) Title() string          { return m.RawTitle }
func (m *Tags) BrainzID() string       { return "" }
func (m *Tags) Artist() string         { return m.RawArtist }
func (m *Tags) Album() string          { return m.RawAlbum }
func (m *Tags) AlbumArtist() string    { return m.RawAlbumArtist }
func (m *Tags) AlbumArtists() []string { return m.RawAlbumArtists }
func (m *Tags) AlbumBrainzID() string  { return "" }
func (m *Tags) Genre() string          { return m.RawGenre }
func (m *Tags) TrackNumber() int       { return 1 }
func (m *Tags) DiscNumber() int        { return 1 }
func (m *Tags) Year() int              { return 2021 }

func (m *Tags) Length() int  { return firstInt(100, m.RawLength) }
func (m *Tags) Bitrate() int { return firstInt(100, m.RawBitrate) }

var _ tags.Parser = (*Tags)(nil)

func first(or string, strs ...string) string {
	for _, str := range strs {
		if str != "" {
			return str
		}
	}
	return or
}

func firstInt(or int, ints ...int) int {
	for _, int := range ints {
		if int > 0 {
			return int
		}
	}
	return or
}
