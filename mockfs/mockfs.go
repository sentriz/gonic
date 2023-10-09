//nolint:thelper
package mockfs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scanner"
	"go.senan.xyz/gonic/tags/tagcommon"
)

var ErrPathNotFound = errors.New("path not found")

type MockFS struct {
	t         testing.TB
	scanner   *scanner.Scanner
	dir       string
	tagReader *tagReader
	db        *db.DB
}

func New(tb testing.TB) *MockFS                        { return newMockFS(tb, []string{""}, "") }
func NewWithDirs(tb testing.TB, dirs []string) *MockFS { return newMockFS(tb, dirs, "") }
func NewWithExcludePattern(tb testing.TB, excludePattern string) *MockFS {
	return newMockFS(tb, []string{""}, excludePattern)
}

func newMockFS(tb testing.TB, dirs []string, excludePattern string) *MockFS {
	tb.Helper()

	dbc, err := db.NewMock()
	if err != nil {
		tb.Fatalf("create db: %v", err)
	}
	tb.Cleanup(func() {
		if err := dbc.Close(); err != nil {
			tb.Fatalf("close db: %v", err)
		}
	})

	if err := dbc.Migrate(db.MigrationContext{}); err != nil {
		tb.Fatalf("migrate db db: %v", err)
	}
	dbc.LogMode(false)

	tmpDir := tb.TempDir()

	var absDirs []string
	for _, dir := range dirs {
		absDirs = append(absDirs, filepath.Join(tmpDir, dir))
	}
	for _, absDir := range absDirs {
		if err := os.MkdirAll(absDir, os.ModePerm); err != nil {
			tb.Fatalf("mk abs dir: %v", err)
		}
	}

	multiValueSettings := map[scanner.Tag]scanner.MultiValueSetting{
		scanner.Genre:       {Mode: scanner.Delim, Delim: ";"},
		scanner.AlbumArtist: {Mode: scanner.Multi},
	}

	tagReader := &tagReader{paths: map[string]*TagInfo{}}
	scanner := scanner.New(absDirs, dbc, multiValueSettings, tagReader, excludePattern)

	return &MockFS{
		t:         tb,
		scanner:   scanner,
		dir:       tmpDir,
		tagReader: tagReader,
		db:        dbc,
	}
}

func (m *MockFS) DB() *db.DB                  { return m.db }
func (m *MockFS) TmpDir() string              { return m.dir }
func (m *MockFS) TagReader() tagcommon.Reader { return m.tagReader }

func (m *MockFS) ScanAndClean() *scanner.Context {
	m.t.Helper()

	ctx, err := m.scanner.ScanAndClean(scanner.ScanOptions{})
	if err != nil {
		m.t.Fatalf("error scan and cleaning: %v", err)
	}
	return ctx
}

func (m *MockFS) ScanAndCleanErr() (*scanner.Context, error) {
	m.t.Helper()

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

func (m *MockFS) AddItems()                              { m.addItems("", "", false) }
func (m *MockFS) AddItemsGlob(onlyGlob string)           { m.addItems("", onlyGlob, false) }
func (m *MockFS) AddItemsPrefix(prefix string)           { m.addItems(prefix, "", false) }
func (m *MockFS) AddItemsWithCovers()                    { m.addItems("", "", true) }
func (m *MockFS) AddItemsPrefixWithCovers(prefix string) { m.addItems(prefix, "", true) }

func (m *MockFS) addItems(prefix string, onlyGlob string, covers bool) {
	p := func(format string, a ...interface{}) string {
		return filepath.Join(prefix, fmt.Sprintf(format, a...))
	}
	for ar := 0; ar < 3; ar++ {
		for al := 0; al < 3; al++ {
			for tr := 0; tr < 3; tr++ {
				path := p("artist-%d/album-%d/track-%d.flac", ar, al, tr)
				if !match(onlyGlob, path) {
					continue
				}

				m.AddTrack(path)
				m.SetTags(path, func(tags *TagInfo) {
					tags.RawArtist = fmt.Sprintf("artist-%d", ar)
					tags.RawAlbumArtist = fmt.Sprintf("artist-%d", ar)
					tags.RawAlbum = fmt.Sprintf("album-%d", al)
					tags.RawTitle = fmt.Sprintf("title-%d", tr)
				})
			}
			if covers {
				path := p("artist-%d/album-%d/cover.png", ar, al)
				if !match(onlyGlob, path) {
					continue
				}

				m.AddCover(path)
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
	m.SetTags(path, func(tags *TagInfo) {
		tags.RawLength = length
		tags.RawBitrate = 0
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
		m.t.Logf("id %-3d root %-3s lr %-15s %-10s pid %-3d cov %-10s",
			album.ID, album.RootDir, album.LeftPath, album.RightPath, album.ParentID, album.Cover)
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

func (m *MockFS) SetTags(path string, cb func(*TagInfo)) {
	absPath := filepath.Join(m.dir, path)
	if err := os.Chtimes(absPath, time.Time{}, time.Now()); err != nil {
		m.t.Fatalf("touch track: %v", err)
	}
	if _, ok := m.tagReader.paths[absPath]; !ok {
		m.tagReader.paths[absPath] = &TagInfo{}
	}
	cb(m.tagReader.paths[absPath])
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
	if err := db.Dump(context.Background(), m.db.DB, destPath); err != nil {
		m.t.Fatalf("dumping db: %v", err)
	}

	m.t.Error(destPath)
}

type tagReader struct {
	paths map[string]*TagInfo
}

func (m *tagReader) CanRead(absPath string) bool {
	stat, _ := os.Stat(absPath)
	return stat.Mode().IsRegular()
}

func (m *tagReader) Read(absPath string) (tagcommon.Info, error) {
	p, ok := m.paths[absPath]
	if !ok {
		return nil, ErrPathNotFound
	}
	if p.Error != nil {
		return nil, p.Error
	}
	return p, nil
}

type TagInfo struct {
	RawTitle        string
	RawArtist       string
	RawAlbum        string
	RawAlbumArtist  string
	RawAlbumArtists []string
	RawGenre        string
	RawBitrate      int
	RawLength       int
	Error           error
}

func (i *TagInfo) Title() string          { return i.RawTitle }
func (i *TagInfo) BrainzID() string       { return "" }
func (i *TagInfo) Artist() string         { return i.RawArtist }
func (i *TagInfo) Album() string          { return i.RawAlbum }
func (i *TagInfo) AlbumArtist() string    { return i.RawAlbumArtist }
func (i *TagInfo) AlbumArtists() []string { return i.RawAlbumArtists }
func (i *TagInfo) AlbumBrainzID() string  { return "" }
func (i *TagInfo) Genre() string          { return i.RawGenre }
func (i *TagInfo) Genres() []string       { return []string{i.RawGenre} }
func (i *TagInfo) TrackNumber() int       { return 1 }
func (i *TagInfo) DiscNumber() int        { return 1 }
func (i *TagInfo) Year() int              { return 2021 }
func (i *TagInfo) Length() int            { return firstInt(100, i.RawLength) }
func (i *TagInfo) Bitrate() int           { return firstInt(100, i.RawBitrate) }

var _ tagcommon.Reader = (*tagReader)(nil)

func firstInt(or int, ints ...int) int {
	for _, int := range ints {
		if int > 0 {
			return int
		}
	}
	return or
}

func match(pattern, name string) bool {
	if pattern == "" {
		return true
	}
	ok, _ := filepath.Match(pattern, name)
	return ok
}
