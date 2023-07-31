package scanner

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/jinzhu/gorm"
	"github.com/rainycape/unidecode"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/mime"
	"go.senan.xyz/gonic/multierr"
	"go.senan.xyz/gonic/scanner/tags"
)

var (
	ErrAlreadyScanning = errors.New("already scanning")
	ErrReadingTags     = errors.New("could not read tags")
)

type Scanner struct {
	db             *db.DB
	musicDirs      []string
	genreSplit     string
	tagger         tags.Reader
	excludePattern *regexp.Regexp
	scanning       *int32
	watcher        *fsnotify.Watcher
	watchMap       map[string]string // maps watched dirs back to root music dir
	watchDone      chan bool
}

func New(musicDirs []string, db *db.DB, genreSplit string, tagger tags.Reader, excludePattern string) *Scanner {
	var excludePatternRegExp *regexp.Regexp
	if excludePattern != "" {
		excludePatternRegExp = regexp.MustCompile(excludePattern)
	}

	return &Scanner{
		db:             db,
		musicDirs:      musicDirs,
		genreSplit:     genreSplit,
		tagger:         tagger,
		excludePattern: excludePatternRegExp,
		scanning:       new(int32),
		watchMap:       make(map[string]string),
		watchDone:      make(chan bool),
	}
}

func (s *Scanner) IsScanning() bool {
	return atomic.LoadInt32(s.scanning) == 1
}
func (s *Scanner) StartScanning() bool {
	return atomic.CompareAndSwapInt32(s.scanning, 0, 1)
}
func (s *Scanner) StopScanning() {
	defer atomic.StoreInt32(s.scanning, 0)
}

type ScanOptions struct {
	IsFull bool
}

func (s *Scanner) ScanAndClean(opts ScanOptions) (*Context, error) {
	if !s.StartScanning() {
		return nil, ErrAlreadyScanning
	}
	defer s.StopScanning()

	start := time.Now()
	c := &Context{
		errs:       &multierr.Err{},
		seenTracks: map[int]struct{}{},
		seenAlbums: map[int]struct{}{},
		isFull:     opts.IsFull,
	}

	log.Println("starting scan")
	defer func() {
		log.Printf("finished scan in %s, +%d/%d tracks (%d err)\n",
			durSince(start), c.SeenTracksNew(), c.SeenTracks(), c.errs.Len())
	}()

	for _, dir := range s.musicDirs {
		err := filepath.WalkDir(dir, func(absPath string, d fs.DirEntry, err error) error {
			return s.scanCallback(c, dir, absPath, d, err)
		})
		if err != nil {
			return nil, fmt.Errorf("walk: %w", err)
		}
	}

	if err := s.cleanTracks(c); err != nil {
		return nil, fmt.Errorf("clean tracks: %w", err)
	}
	if err := s.cleanAlbums(c); err != nil {
		return nil, fmt.Errorf("clean albums: %w", err)
	}
	if err := s.cleanArtists(c); err != nil {
		return nil, fmt.Errorf("clean artists: %w", err)
	}
	if err := s.cleanGenres(c); err != nil {
		return nil, fmt.Errorf("clean genres: %w", err)
	}

	if err := s.db.SetSetting("last_scan_time", strconv.FormatInt(time.Now().Unix(), 10)); err != nil {
		return nil, fmt.Errorf("set scan time: %w", err)
	}

	if c.errs.Len() > 0 {
		return c, c.errs
	}

	return c, nil
}

func (s *Scanner) ExecuteWatch() error {
	var err error
	s.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		log.Printf("error creating watcher: %v\n", err)
		return err
	}
	defer s.watcher.Close()

	t := time.NewTimer(10 * time.Second)
	if !t.Stop() {
		<-t.C
	}

	for _, dir := range s.musicDirs {
		err := filepath.WalkDir(dir, func(absPath string, d fs.DirEntry, err error) error {
			return s.watchCallback(dir, absPath, d, err)
		})
		if err != nil {
			log.Printf("error watching directory tree: %v\n", err)
		}
	}

	scanList := map[string]struct{}{}
	for {
		select {
		case <-t.C:
			if !s.StartScanning() {
				scanList = map[string]struct{}{}
				break
			}
			for dirName := range scanList {
				c := &Context{
					errs:       &multierr.Err{},
					seenTracks: map[int]struct{}{},
					seenAlbums: map[int]struct{}{},
					isFull:     false,
				}
				musicDirName := s.watchMap[dirName]
				if musicDirName == "" {
					musicDirName = s.watchMap[filepath.Dir(dirName)]
				}
				err = filepath.WalkDir(dirName, func(absPath string, d fs.DirEntry, err error) error {
					return s.watchCallback(musicDirName, absPath, d, err)
				})
				if err != nil {
					log.Printf("error watching directory tree: %v\n", err)
				}
				err = filepath.WalkDir(dirName, func(absPath string, d fs.DirEntry, err error) error {
					return s.scanCallback(c, musicDirName, absPath, d, err)
				})
				if err != nil {
					log.Printf("error walking: %v", err)
				}

			}
			scanList = map[string]struct{}{}
			s.StopScanning()
		case event := <-s.watcher.Events:
			var dirName string
			if event.Op&(fsnotify.Create|fsnotify.Write) == 0 {
				break
			}
			if len(scanList) == 0 {
				t.Reset(10 * time.Second)
			}
			fileInfo, err := os.Stat(event.Name)
			if err != nil {
				break
			}
			if fileInfo.IsDir() {
				dirName = event.Name
			} else {
				dirName = filepath.Dir(event.Name)
			}
			scanList[dirName] = struct{}{}
		case err = <-s.watcher.Errors:
			log.Printf("error from watcher: %v\n", err)
		case <-s.watchDone:
			return nil
		}
	}
}

func (s *Scanner) CancelWatch() {
	s.watchDone <- true
}

func (s *Scanner) watchCallback(dir string, absPath string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	switch d.Type() {
	case os.ModeDir:
	case os.ModeSymlink:
		eval, _ := filepath.EvalSymlinks(absPath)
		return filepath.WalkDir(eval, func(subAbs string, d fs.DirEntry, err error) error {
			subAbs = strings.Replace(subAbs, eval, absPath, 1)
			return s.watchCallback(dir, subAbs, d, err)
		})
	default:
		return nil
	}

	if s.watchMap[absPath] == "" {
		s.watchMap[absPath] = dir
		err = s.watcher.Add(absPath)
	}
	return err
}

func (s *Scanner) scanCallback(c *Context, dir string, absPath string, d fs.DirEntry, err error) error {
	if err != nil {
		c.errs.Add(err)
		return nil
	}
	if dir == absPath {
		return nil
	}

	switch d.Type() {
	case os.ModeDir:
	case os.ModeSymlink:
		eval, _ := filepath.EvalSymlinks(absPath)
		return filepath.WalkDir(eval, func(subAbs string, d fs.DirEntry, err error) error {
			subAbs = strings.Replace(subAbs, eval, absPath, 1)
			return s.scanCallback(c, dir, subAbs, d, err)
		})
	default:
		return nil
	}

	if s.excludePattern != nil && s.excludePattern.MatchString(absPath) {
		log.Printf("excluding folder `%s`", absPath)
		return nil
	}

	log.Printf("processing folder `%s`", absPath)

	tx := s.db.Begin()
	if err := s.scanDir(tx, c, dir, absPath); err != nil {
		c.errs.Add(fmt.Errorf("%q: %w", absPath, err))
		tx.Rollback()
		return nil
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (s *Scanner) scanDir(tx *db.DB, c *Context, musicDir string, absPath string) error {
	items, err := os.ReadDir(absPath)
	if err != nil {
		return err
	}

	var tracks []string
	var cover string
	for _, item := range items {
		fullpath := filepath.Join(absPath, item.Name())
		if s.excludePattern != nil && s.excludePattern.MatchString(fullpath) {
			log.Printf("excluding path `%s`", fullpath)
			continue
		}

		if isCover(item.Name()) {
			cover = item.Name()
			continue
		}
		if mime := mime.TypeByAudioExtension(filepath.Ext(item.Name())); mime != "" {
			tracks = append(tracks, item.Name())
			continue
		}
	}

	relPath, _ := filepath.Rel(musicDir, absPath)
	pdir, pbasename := filepath.Split(filepath.Dir(relPath))
	var parent db.Album
	if err := tx.Where("root_dir=? AND left_path=? AND right_path=?", musicDir, pdir, pbasename).Assign(db.Album{RootDir: musicDir, LeftPath: pdir, RightPath: pbasename}).FirstOrCreate(&parent).Error; err != nil {
		return fmt.Errorf("first or create parent: %w", err)
	}

	c.seenAlbums[parent.ID] = struct{}{}

	dir, basename := filepath.Split(relPath)
	var album db.Album
	if err := populateAlbumBasics(tx, musicDir, &parent, &album, dir, basename, cover); err != nil {
		return fmt.Errorf("populate album basics: %w", err)
	}

	c.seenAlbums[album.ID] = struct{}{}

	sort.Strings(tracks)
	for i, basename := range tracks {
		absPath := filepath.Join(musicDir, relPath, basename)
		if err := s.populateTrackAndAlbumArtists(tx, c, i, &parent, &album, basename, absPath); err != nil {
			return fmt.Errorf("populate track %q: %w", basename, err)
		}
	}

	return nil
}

func (s *Scanner) populateTrackAndAlbumArtists(tx *db.DB, c *Context, i int, parent, album *db.Album, basename string, absPath string) error {
	stat, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stating %q: %w", basename, err)
	}

	var track db.Track
	if err := tx.Where("album_id=? AND filename=?", album.ID, filepath.Base(basename)).First(&track).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query track: %w", err)
	}

	if !c.isFull && track.ID != 0 && stat.ModTime().Before(track.UpdatedAt) {
		c.seenTracks[track.ID] = struct{}{}
		return nil
	}

	trags, err := s.tagger.Read(absPath)
	if err != nil {
		return fmt.Errorf("%v: %w", err, ErrReadingTags)
	}

	genreNames := strings.Split(tags.MustGenre(trags), s.genreSplit)
	genreIDs, err := populateGenres(tx, genreNames)
	if err != nil {
		return fmt.Errorf("populate genres: %w", err)
	}

	// metadata for the album table comes only from the the first track's tags
	if i == 0 || album.TagArtist == nil {
		albumArtist, err := populateAlbumArtist(tx, parent, tags.MustAlbumArtist(trags))
		if err != nil {
			return fmt.Errorf("populate album artist: %w", err)
		}
		if err := populateAlbum(tx, album, albumArtist, trags, stat.ModTime()); err != nil {
			return fmt.Errorf("populate album: %w", err)
		}
		if err := populateAlbumGenres(tx, album, genreIDs); err != nil {
			return fmt.Errorf("populate album genres: %w", err)
		}
	}

	if err := populateTrack(tx, album, &track, trags, basename, int(stat.Size())); err != nil {
		return fmt.Errorf("process %q: %w", basename, err)
	}
	if err := populateTrackGenres(tx, &track, genreIDs); err != nil {
		return fmt.Errorf("populate track genres: %w", err)
	}

	c.seenTracks[track.ID] = struct{}{}
	c.seenTracksNew++

	return nil
}

func populateAlbum(tx *db.DB, album *db.Album, albumArtist *db.Artist, trags tags.Parser, modTime time.Time) error {
	albumName := tags.MustAlbum(trags)
	album.TagTitle = albumName
	album.TagTitleUDec = decoded(albumName)
	album.TagBrainzID = trags.AlbumBrainzID()
	album.TagYear = trags.Year()
	album.TagArtist = albumArtist

	album.ModifiedAt = modTime
	album.CreatedAt = modTime

	if err := tx.Save(&album).Error; err != nil {
		return fmt.Errorf("saving album: %w", err)
	}

	return nil
}

func populateAlbumBasics(tx *db.DB, musicDir string, parent, album *db.Album, dir, basename string, cover string) error {
	if err := tx.Where("root_dir=? AND left_path=? AND right_path=?", musicDir, dir, basename).First(album).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("find album: %w", err)
	}

	// see if we can save ourselves from an extra write if it's found and nothing has changed
	if album.ID != 0 && album.Cover == cover && album.ParentID == parent.ID {
		return nil
	}

	album.RootDir = musicDir
	album.LeftPath = dir
	album.RightPath = basename
	album.Cover = cover
	album.RightPathUDec = decoded(basename)
	album.ParentID = parent.ID

	if err := tx.Save(&album).Error; err != nil {
		return fmt.Errorf("saving album: %w", err)
	}

	return nil
}

func populateTrack(tx *db.DB, album *db.Album, track *db.Track, trags tags.Parser, absPath string, size int) error {
	basename := filepath.Base(absPath)
	track.Filename = basename
	track.FilenameUDec = decoded(basename)
	track.Size = size
	track.AlbumID = album.ID
	track.ArtistID = album.TagArtist.ID

	track.TagTitle = trags.Title()
	track.TagTitleUDec = decoded(trags.Title())
	track.TagTrackArtist = trags.Artist()
	track.TagTrackNumber = trags.TrackNumber()
	track.TagDiscNumber = trags.DiscNumber()
	track.TagBrainzID = trags.BrainzID()

	track.Length = trags.Length()   // these two should be calculated
	track.Bitrate = trags.Bitrate() // ...from the file instead of tags

	if err := tx.Save(&track).Error; err != nil {
		return fmt.Errorf("saving track: %w", err)
	}

	return nil
}

func populateAlbumArtist(tx *db.DB, parent *db.Album, artistName string) (*db.Artist, error) {
	var update db.Artist
	update.Name = artistName
	update.NameUDec = decoded(artistName)
	if parent.Cover != "" {
		update.Cover = parent.Cover
	}
	var artist db.Artist
	if err := tx.Where("name=?", artistName).Assign(update).FirstOrCreate(&artist).Error; err != nil {
		return nil, fmt.Errorf("find or create artist: %w", err)
	}
	return &artist, nil
}

func populateGenres(tx *db.DB, names []string) ([]int, error) {
	var filteredNames []string
	for _, name := range names {
		if clean := strings.TrimSpace(name); clean != "" {
			filteredNames = append(filteredNames, clean)
		}
	}
	if len(filteredNames) == 0 {
		return []int{}, nil
	}
	var ids []int
	for _, name := range filteredNames {
		var genre db.Genre
		if err := tx.FirstOrCreate(&genre, db.Genre{Name: name}).Error; err != nil {
			return nil, fmt.Errorf("find or create genre: %w", err)
		}
		ids = append(ids, genre.ID)
	}
	return ids, nil
}

func populateTrackGenres(tx *db.DB, track *db.Track, genreIDs []int) error {
	if err := tx.Where("track_id=?", track.ID).Delete(db.TrackGenre{}).Error; err != nil {
		return fmt.Errorf("delete old track genre records: %w", err)
	}

	if err := tx.InsertBulkLeftMany("track_genres", []string{"track_id", "genre_id"}, track.ID, genreIDs); err != nil {
		return fmt.Errorf("insert bulk track genres: %w", err)
	}
	return nil
}

func populateAlbumGenres(tx *db.DB, album *db.Album, genreIDs []int) error {
	if err := tx.Where("album_id=?", album.ID).Delete(db.AlbumGenre{}).Error; err != nil {
		return fmt.Errorf("delete old album genre records: %w", err)
	}

	if err := tx.InsertBulkLeftMany("album_genres", []string{"album_id", "genre_id"}, album.ID, genreIDs); err != nil {
		return fmt.Errorf("insert bulk album genres: %w", err)
	}
	return nil
}

func (s *Scanner) cleanTracks(c *Context) error {
	start := time.Now()
	defer func() { log.Printf("finished clean tracks in %s, %d removed", durSince(start), c.TracksMissing()) }()

	var all []int
	err := s.db.
		Model(&db.Track{}).
		Pluck("id", &all).
		Error
	if err != nil {
		return fmt.Errorf("plucking ids: %w", err)
	}
	for _, a := range all {
		if _, ok := c.seenTracks[a]; !ok {
			c.tracksMissing = append(c.tracksMissing, int64(a))
		}
	}
	return s.db.TransactionChunked(c.tracksMissing, func(tx *gorm.DB, chunk []int64) error {
		return tx.Where(chunk).Delete(&db.Track{}).Error
	})
}

func (s *Scanner) cleanAlbums(c *Context) error {
	start := time.Now()
	defer func() { log.Printf("finished clean albums in %s, %d removed", durSince(start), c.AlbumsMissing()) }()

	var all []int
	err := s.db.
		Model(&db.Album{}).
		Pluck("id", &all).
		Error
	if err != nil {
		return fmt.Errorf("plucking ids: %w", err)
	}
	for _, a := range all {
		if _, ok := c.seenAlbums[a]; !ok {
			c.albumsMissing = append(c.albumsMissing, int64(a))
		}
	}
	return s.db.TransactionChunked(c.albumsMissing, func(tx *gorm.DB, chunk []int64) error {
		return tx.Where(chunk).Delete(&db.Album{}).Error
	})
}

func (s *Scanner) cleanArtists(c *Context) error {
	start := time.Now()
	defer func() { log.Printf("finished clean artists in %s, %d removed", durSince(start), c.ArtistsMissing()) }()

	sub := s.db.
		Select("artists.id").
		Model(&db.Artist{}).
		Joins("LEFT JOIN albums ON albums.tag_artist_id=artists.id").
		Where("albums.id IS NULL").
		SubQuery()
	q := s.db.
		Where("artists.id IN ?", sub).
		Delete(&db.Artist{})
	if err := q.Error; err != nil {
		return err
	}
	c.artistsMissing = int(q.RowsAffected)
	return nil
}

func (s *Scanner) cleanGenres(c *Context) error {
	start := time.Now()
	defer func() { log.Printf("finished clean genres in %s, %d removed", durSince(start), c.GenresMissing()) }()

	subTrack := s.db.
		Select("genres.id").
		Model(&db.Genre{}).
		Joins("LEFT JOIN track_genres ON track_genres.genre_id=genres.id").
		Where("track_genres.genre_id IS NULL").
		SubQuery()
	subAlbum := s.db.
		Select("genres.id").
		Model(&db.Genre{}).
		Joins("LEFT JOIN album_genres ON album_genres.genre_id=genres.id").
		Where("album_genres.genre_id IS NULL").
		SubQuery()
	q := s.db.
		Where("genres.id IN ? AND genres.id IN ?", subTrack, subAlbum).
		Delete(&db.Genre{})
	c.genresMissing = int(q.RowsAffected)
	return nil
}

//nolint:gochecknoglobals
var coverNames = map[string]struct{}{}

//nolint:gochecknoinits
func init() {
	for _, name := range []string{"cover", "folder", "front", "albumart", "album", "artist"} {
		for _, ext := range []string{"jpg", "jpeg", "png", "bmp", "gif"} {
			coverNames[fmt.Sprintf("%s.%s", name, ext)] = struct{}{}
			for i := 0; i < 3; i++ {
				coverNames[fmt.Sprintf("%s.%d.%s", name, i, ext)] = struct{}{} // support beets extras
			}
		}
	}
}

func isCover(name string) bool {
	_, ok := coverNames[strings.ToLower(name)]
	return ok
}

// decoded converts a string to it's latin equivalent.
// it will be used by the model's *UDec fields, and is only set if it
// differs from the original. the fields are used for searching.
func decoded(in string) string {
	if u := unidecode.Unidecode(in); u != in {
		return u
	}
	return ""
}

func durSince(t time.Time) time.Duration {
	return time.Since(t).Truncate(10 * time.Microsecond)
}

type Context struct {
	errs   *multierr.Err
	isFull bool

	seenTracks    map[int]struct{}
	seenAlbums    map[int]struct{}
	seenTracksNew int

	tracksMissing  []int64
	albumsMissing  []int64
	artistsMissing int
	genresMissing  int
}

func (c *Context) SeenTracks() int    { return len(c.seenTracks) }
func (c *Context) SeenAlbums() int    { return len(c.seenAlbums) }
func (c *Context) SeenTracksNew() int { return c.seenTracksNew }

func (c *Context) TracksMissing() int  { return len(c.tracksMissing) }
func (c *Context) AlbumsMissing() int  { return len(c.albumsMissing) }
func (c *Context) ArtistsMissing() int { return c.artistsMissing }
func (c *Context) GenresMissing() int  { return c.genresMissing }
