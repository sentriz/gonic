//nolint:nestif
package scanner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/djherbis/times"
	"github.com/fsnotify/fsnotify"
	"github.com/jinzhu/gorm"
	"github.com/rainycape/unidecode"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/fileutil"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/tags"
	"go.senan.xyz/wrtag/coverparse"
	"go.senan.xyz/wrtag/tags/normtag"
)

var (
	ErrAlreadyScanning = errors.New("already scanning")
	ErrReadingTags     = errors.New("could not read tags")
)

type Scanner struct {
	db                 *db.DB
	musicDirs          []string
	multiValueSettings map[Tag]MultiValueSetting
	tagReader          tags.Reader
	excludePattern     *regexp.Regexp
	scanning           *int32
}

func New(musicDirs []string, db *db.DB, multiValueSettings map[Tag]MultiValueSetting, tagReader tags.Reader, excludePattern string) *Scanner {
	var excludePatternRegExp *regexp.Regexp
	if excludePattern != "" {
		excludePatternRegExp = regexp.MustCompile(excludePattern)
	}

	return &Scanner{
		db:                 db,
		musicDirs:          musicDirs,
		multiValueSettings: multiValueSettings,
		tagReader:          tagReader,
		excludePattern:     excludePatternRegExp,
		scanning:           new(int32),
	}
}

func (s *Scanner) IsScanning() bool    { return atomic.LoadInt32(s.scanning) == 1 }
func (s *Scanner) StartScanning() bool { return atomic.CompareAndSwapInt32(s.scanning, 0, 1) }
func (s *Scanner) StopScanning()       { defer atomic.StoreInt32(s.scanning, 0) }

type ScanOptions struct {
	IsFull bool
}

func (s *Scanner) ScanAndClean(opts ScanOptions) (*State, error) {
	if !s.StartScanning() {
		return nil, ErrAlreadyScanning
	}
	defer s.StopScanning()

	start := time.Now()
	st := &State{
		seenTracks: map[int]struct{}{},
		seenAlbums: map[int]struct{}{},
		isFull:     opts.IsFull,
	}

	log.Println("starting scan")
	defer func() {
		log.Printf("finished scan in %s, +%d/%d tracks (%d err)\n",
			durSince(start), st.SeenTracksNew(), st.SeenTracks(), len(st.errs))
	}()

	for _, dir := range s.musicDirs {
		err := filepath.WalkDir(dir, func(absPath string, d fs.DirEntry, err error) error {
			return s.scanCallback(st, absPath, d, err)
		})
		if err != nil {
			return nil, fmt.Errorf("walk: %w", err)
		}
	}

	if err := s.cleanTracks(st); err != nil {
		return nil, fmt.Errorf("clean tracks: %w", err)
	}
	if err := s.cleanAlbums(st); err != nil {
		return nil, fmt.Errorf("clean albums: %w", err)
	}
	if err := s.cleanArtists(st); err != nil {
		return nil, fmt.Errorf("clean artists: %w", err)
	}
	if err := s.cleanGenres(st); err != nil {
		return nil, fmt.Errorf("clean genres: %w", err)
	}
	if err := s.cleanBookmarks(st); err != nil {
		return nil, fmt.Errorf("clean bookmarks: %w", err)
	}

	if err := s.db.SetSetting(db.LastScanTime, strconv.FormatInt(time.Now().Unix(), 10)); err != nil {
		return nil, fmt.Errorf("set scan time: %w", err)
	}

	return st, errors.Join(st.errs...)
}

func (s *Scanner) ExecuteWatch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}
	defer watcher.Close()

	const batchInterval = 10 * time.Second
	batchT := time.NewTimer(batchInterval)
	batchT.Stop()

	for _, dir := range s.musicDirs {
		err := filepath.WalkDir(dir, func(absPath string, d fs.DirEntry, err error) error {
			return watchCallback(watcher, absPath, d, err)
		})
		if err != nil {
			log.Printf("error watching directory tree: %v\n", err)
			continue
		}
	}

	batchSeen := map[string]struct{}{}
	batchClean := false
	for {
		select {
		case <-batchT.C:
			if batchClean {
				if _, err := s.ScanAndClean(ScanOptions{}); err != nil {
					log.Printf("error scanning: %v", err)
				}
				clear(batchSeen)
				batchClean = false
				break
			}
			if !s.StartScanning() {
				break
			}
			for absPath := range batchSeen {
				st := &State{
					seenTracks: map[int]struct{}{},
					seenAlbums: map[int]struct{}{},
				}
				err := filepath.WalkDir(absPath, func(absPath string, d fs.DirEntry, err error) error {
					return watchCallback(watcher, absPath, d, err)
				})
				if err != nil {
					log.Printf("error watching directory tree: %v\n", err)
					continue
				}
				err = filepath.WalkDir(absPath, func(absPath string, d fs.DirEntry, err error) error {
					return s.scanCallback(st, absPath, d, err)
				})
				if err != nil {
					log.Printf("error walking: %v", err)
					continue
				}
			}
			s.StopScanning()
			clear(batchSeen)

		case event := <-watcher.Events:
			if event.Op&(fsnotify.Remove) == fsnotify.Remove {
				batchClean = true
				break
			}
			if event.Op&(fsnotify.Create|fsnotify.Write) == 0 {
				break
			}
			fileInfo, err := os.Stat(event.Name)
			if err != nil {
				break
			}
			if fileInfo.IsDir() {
				batchSeen[event.Name] = struct{}{}
			} else {
				batchSeen[filepath.Dir(event.Name)] = struct{}{}
			}
			batchT.Reset(batchInterval)

		case err := <-watcher.Errors:
			log.Printf("error from watcher: %v\n", err)

		case <-ctx.Done():
			return nil
		}
	}
}

func watchCallback(watcher *fsnotify.Watcher, absPath string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	switch d.Type() {
	case os.ModeDir:
	case os.ModeSymlink:
		return symWalk(absPath, func(subAbs string, d fs.DirEntry, err error) error {
			return watchCallback(watcher, subAbs, d, err)
		})
	default:
		return nil
	}

	if err := watcher.Add(absPath); err != nil {
		return fmt.Errorf("add path to watcher: %w", err)
	}
	return nil
}

func (s *Scanner) scanCallback(st *State, absPath string, d fs.DirEntry, err error) error {
	if err != nil {
		st.errs = append(st.errs, err)
		return nil
	}

	switch d.Type() {
	case os.ModeDir:
	case os.ModeSymlink:
		return symWalk(absPath, func(subAbs string, d fs.DirEntry, err error) error {
			return s.scanCallback(st, subAbs, d, err)
		})
	default:
		return nil
	}

	if s.excludePattern != nil && s.excludePattern.MatchString(absPath) {
		log.Printf("excluding folder %q", absPath)
		return nil
	}

	log.Printf("processing folder %q", absPath)

	if err := s.scanDir(st, absPath); err != nil {
		st.errs = append(st.errs, fmt.Errorf("%q: %w", absPath, err))
		return nil
	}

	return nil
}

func (s *Scanner) scanDir(st *State, absPath string) error {
	musicDir, relPath := musicDirRelative(s.musicDirs, absPath)
	if musicDir == absPath {
		return nil
	}

	items, err := os.ReadDir(absPath)
	if err != nil {
		return err
	}

	var trackPaths []string
	var cover string
	for _, item := range items {
		absPath := filepath.Join(absPath, item.Name())
		if s.excludePattern != nil && s.excludePattern.MatchString(absPath) {
			log.Printf("excluding path %q", absPath)
			continue
		}
		if item.IsDir() {
			continue
		}

		if coverparse.IsCover(item.Name()) {
			cover = coverparse.BestBetween(cover, item.Name())
			continue
		}
		if s.tagReader.CanRead(absPath) {
			trackPaths = append(trackPaths, item.Name())
			continue
		}
	}

	pdir, pbasename := filepath.Split(filepath.Dir(relPath))
	var parent db.Album
	if err := s.db.Where("root_dir=? AND left_path=? AND right_path=?", musicDir, pdir, pbasename).Assign(db.Album{RootDir: musicDir, LeftPath: pdir, RightPath: pbasename}).FirstOrCreate(&parent).Error; err != nil {
		return fmt.Errorf("first or create parent: %w", err)
	}

	st.seenAlbums[parent.ID] = struct{}{}

	dir, basename := filepath.Split(relPath)
	var album db.Album
	if err := populateAlbumBasics(s.db, musicDir, &parent, &album, dir, basename, cover); err != nil {
		return fmt.Errorf("populate album basics: %w", err)
	}

	st.seenAlbums[album.ID] = struct{}{}

	if len(trackPaths) == 0 {
		return nil
	}

	var tracks []*db.Track
	if err := s.db.Where("album_id=? AND filename IN (?)", album.ID, trackPaths).Find(&tracks).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query track: %w", err)
	}

	trackMap := make(map[string]*db.Track, len(tracks))
	for _, t := range tracks {
		trackMap[t.Filename] = t
		st.seenTracks[t.ID] = struct{}{}
	}

	type trackUpdate struct {
		i        int
		basename string
		absPath  string
		track    *db.Track
		timeSpec times.Timespec
	}
	trackUpdates := make([]trackUpdate, 0, len(trackPaths))

	sort.Strings(trackPaths)

	for i, basename := range trackPaths {
		absPath := filepath.Join(musicDir, relPath, basename)

		timeSpec, err := times.Stat(absPath)
		if err != nil {
			return fmt.Errorf("get times %q: %w", basename, err)
		}

		// might be nil if new track
		track := trackMap[basename]

		if st.isFull || track == nil || timeSpec.ModTime().After(track.UpdatedAt) {
			trackUpdates = append(trackUpdates, trackUpdate{
				i:        i,
				basename: basename,
				absPath:  absPath,
				track:    track,
				timeSpec: timeSpec,
			})
		}
	}

	if len(trackUpdates) == 0 {
		return nil
	}

	return s.db.Transaction(func(tx *db.DB) error {
		for _, t := range trackUpdates {
			if err := s.populateTrackAndArtists(tx, st, t.i, &album, t.track, t.timeSpec, t.basename, t.absPath); err != nil {
				return fmt.Errorf("populate track %q: %w", t.basename, err)
			}
		}
		return nil
	})
}

func (s *Scanner) populateTrackAndArtists(tx *db.DB, st *State, i int, album *db.Album, track *db.Track, timeSpec times.Timespec, basename, absPath string) error {
	trprops, trags, err := s.tagReader.Read(absPath)
	if err != nil {
		return fmt.Errorf("%w: %w", err, ErrReadingTags)
	}

	genreNames := ParseMulti(s.multiValueSettings[Genre], tags.MustGenres(trags), tags.MustGenre(trags))
	genreIDs, err := populateGenres(tx, genreNames)
	if err != nil {
		return fmt.Errorf("populate genres: %w", err)
	}

	// metadata for the album table comes only from the first track's tags
	if i == 0 {
		if err := tx.Where("album_id=?", album.ID).Delete(db.ArtistAppearances{}).Error; err != nil {
			return fmt.Errorf("delete artist appearances: %w", err)
		}

		albumArtistNames := ParseMulti(s.multiValueSettings[AlbumArtist], tags.MustAlbumArtists(trags), tags.MustAlbumArtist(trags))
		var albumArtistIDs []int
		for _, albumArtistName := range albumArtistNames {
			albumArtist, err := populateArtist(tx, albumArtistName)
			if err != nil {
				return fmt.Errorf("populate album artist: %w", err)
			}
			albumArtistIDs = append(albumArtistIDs, albumArtist.ID)
		}
		if err := populateAlbumArtists(tx, album, albumArtistIDs); err != nil {
			return fmt.Errorf("populate album artists: %w", err)
		}

		if err := populateArtistAppearances(tx, album, albumArtistIDs); err != nil {
			return fmt.Errorf("populate track artists: %w", err)
		}

		modTime, createTime := timeSpec.ModTime(), timeSpec.ModTime()
		if timeSpec.HasBirthTime() {
			createTime = timeSpec.BirthTime()
		}
		if err := populateAlbum(tx, album, trags, modTime, createTime); err != nil {
			return fmt.Errorf("populate album: %w", err)
		}

		if err := populateAlbumGenres(tx, album, genreIDs); err != nil {
			return fmt.Errorf("populate album genres: %w", err)
		}
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stating %q: %w", basename, err)
	}

	if track == nil {
		track = &db.Track{}
	}

	if err := populateTrack(tx, album, track, trprops, trags, basename, int(stat.Size())); err != nil {
		return fmt.Errorf("process %q: %w", basename, err)
	}
	if err := populateTrackGenres(tx, track, genreIDs); err != nil {
		return fmt.Errorf("populate track genres: %w", err)
	}

	trackArtistNames := ParseMulti(s.multiValueSettings[Artist], tags.MustArtists(trags), tags.MustArtist(trags))
	var trackArtistIDs []int
	for _, trackArtistName := range trackArtistNames {
		trackArtist, err := populateArtist(tx, trackArtistName)
		if err != nil {
			return fmt.Errorf("populate track artist: %w", err)
		}
		trackArtistIDs = append(trackArtistIDs, trackArtist.ID)
	}
	if err := populateTrackArtists(tx, track, trackArtistIDs); err != nil {
		return fmt.Errorf("populate track artists: %w", err)
	}

	if err := populateArtistAppearances(tx, album, trackArtistIDs); err != nil {
		return fmt.Errorf("populate track artists: %w", err)
	}

	st.seenTracks[track.ID] = struct{}{}
	st.seenTracksNew++

	return nil
}

func populateAlbum(tx *db.DB, album *db.Album, trags map[string][]string, modTime, createTime time.Time) error {
	albumName := tags.MustAlbum(trags)
	album.TagTitle = albumName
	album.TagTitleUDec = decoded(albumName)
	album.TagAlbumArtist = tags.MustAlbumArtist(trags)
	album.TagBrainzID = normtag.Get(trags, normtag.MusicBrainzReleaseID)
	album.TagYear = tags.MustYear(trags)
	album.TagCompilation = tags.ParseBool(normtag.Get(trags, normtag.Compilation))
	album.TagReleaseType = normtag.Get(trags, normtag.ReleaseType)

	album.ModifiedAt = modTime
	if album.CreatedAt.After(createTime) {
		album.CreatedAt = createTime // reset created at to match filesytem for new albums
	}

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

func populateTrack(tx *db.DB, album *db.Album, track *db.Track, trprops tags.Properties, trags map[string][]string, absPath string, size int) error {
	basename := filepath.Base(absPath)
	track.Filename = basename
	track.FilenameUDec = decoded(basename)
	track.Size = size
	track.AlbumID = album.ID
	track.TagLyrics = normtag.Get(trags, normtag.Lyrics)

	trackTitle := normtag.Get(trags, normtag.Title)
	track.TagTitle = trackTitle
	track.TagTitleUDec = decoded(trackTitle)
	track.TagTrackArtist = tags.MustArtist(trags)
	track.TagTrackNumber = tags.ParseInt(normtag.Get(trags, normtag.TrackNumber))
	track.TagDiscNumber = tags.ParseInt(normtag.Get(trags, normtag.DiscNumber))
	track.TagBrainzID = normtag.Get(trags, normtag.MusicBrainzRecordingID)

	track.ReplayGainTrackGain = tags.ParseDB(normtag.Get(trags, normtag.ReplayGainTrackGain))
	track.ReplayGainTrackPeak = tags.ParseFloat(normtag.Get(trags, normtag.ReplayGainTrackPeak))
	track.ReplayGainAlbumGain = tags.ParseDB(normtag.Get(trags, normtag.ReplayGainAlbumGain))
	track.ReplayGainAlbumPeak = tags.ParseFloat(normtag.Get(trags, normtag.ReplayGainAlbumPeak))

	// these two are calculated from the file instead of tags
	track.Length = int(trprops.Length.Seconds())
	track.Bitrate = int(trprops.Bitrate)

	if err := tx.Save(&track).Error; err != nil {
		return fmt.Errorf("saving track: %w", err)
	}

	return nil
}

func populateArtist(tx *db.DB, artistName string) (*db.Artist, error) {
	var update db.Artist
	update.Name = artistName
	update.NameUDec = decoded(artistName)
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

func populateAlbumArtists(tx *db.DB, album *db.Album, albumArtistIDs []int) error {
	if err := tx.Where("album_id=?", album.ID).Delete(db.AlbumArtist{}).Error; err != nil {
		return fmt.Errorf("delete old album artists: %w", err)
	}

	if err := tx.InsertBulkLeftMany("album_artists", []string{"album_id", "artist_id"}, album.ID, albumArtistIDs); err != nil {
		return fmt.Errorf("insert bulk album artists: %w", err)
	}
	return nil
}

func populateTrackArtists(tx *db.DB, track *db.Track, trackArtistIDs []int) error {
	if err := tx.Where("track_id=?", track.ID).Delete(db.TrackArtist{}).Error; err != nil {
		return fmt.Errorf("delete old track artists: %w", err)
	}

	if err := tx.InsertBulkLeftMany("track_artists", []string{"track_id", "artist_id"}, track.ID, trackArtistIDs); err != nil {
		return fmt.Errorf("insert bulk track artists: %w", err)
	}
	return nil
}

func populateArtistAppearances(tx *db.DB, album *db.Album, artistIDs []int) error {
	if err := tx.InsertBulkLeftMany("artist_appearances", []string{"album_id", "artist_id"}, album.ID, artistIDs); err != nil {
		return fmt.Errorf("insert bulk track artists: %w", err)
	}
	return nil
}

func (s *Scanner) cleanTracks(st *State) error {
	start := time.Now()
	defer func() { log.Printf("finished clean tracks in %s, %d removed", durSince(start), st.TracksMissing()) }()

	var all []int
	err := s.db.
		Model(&db.Track{}).
		Pluck("id", &all).
		Error
	if err != nil {
		return fmt.Errorf("plucking ids: %w", err)
	}
	for _, a := range all {
		if _, ok := st.seenTracks[a]; !ok {
			st.tracksMissing = append(st.tracksMissing, int64(a))
		}
	}
	return s.db.TransactionChunked(st.tracksMissing, func(tx *db.DB, chunk []int64) error {
		return tx.Where(chunk).Delete(&db.Track{}).Error
	})
}

func (s *Scanner) cleanAlbums(st *State) error {
	start := time.Now()
	defer func() { log.Printf("finished clean albums in %s, %d removed", durSince(start), st.AlbumsMissing()) }()

	var all []int
	err := s.db.
		Model(&db.Album{}).
		Pluck("id", &all).
		Error
	if err != nil {
		return fmt.Errorf("plucking ids: %w", err)
	}
	for _, a := range all {
		if _, ok := st.seenAlbums[a]; !ok {
			st.albumsMissing = append(st.albumsMissing, int64(a))
		}
	}
	return s.db.TransactionChunked(st.albumsMissing, func(tx *db.DB, chunk []int64) error {
		return tx.Where(chunk).Delete(&db.Album{}).Error
	})
}

func (s *Scanner) cleanArtists(st *State) error {
	start := time.Now()
	defer func() { log.Printf("finished clean artists in %s, %d removed", durSince(start), st.ArtistsMissing()) }()

	// gorm doesn't seem to support subqueries without parens for UNION
	q := s.db.Exec(`
		DELETE FROM artists
		WHERE id NOT IN (
			SELECT artist_id FROM track_artists
			UNION
			SELECT artist_id FROM album_artists
			UNION
			SELECT artist_id FROM artist_appearances
		)
    `)
	if err := q.Error; err != nil {
		return err
	}
	st.artistsMissing = int(q.RowsAffected)
	return nil
}

func (s *Scanner) cleanGenres(st *State) error { //nolint:unparam
	start := time.Now()
	defer func() { log.Printf("finished clean genres in %s, %d removed", durSince(start), st.GenresMissing()) }()

	subTrack := s.db.
		Select("genres.id").
		Model(db.Genre{}).
		Joins("LEFT JOIN track_genres ON track_genres.genre_id=genres.id").
		Where("track_genres.genre_id IS NULL").
		SubQuery()
	subAlbum := s.db.
		Select("genres.id").
		Model(db.Genre{}).
		Joins("LEFT JOIN album_genres ON album_genres.genre_id=genres.id").
		Where("album_genres.genre_id IS NULL").
		SubQuery()
	q := s.db.
		Where("genres.id IN ? AND genres.id IN ?", subTrack, subAlbum).
		Delete(db.Genre{})
	st.genresMissing += int(q.RowsAffected)

	subAlbumGenresNoTracks := s.db.
		Select("album_genres.genre_id").
		Model(db.AlbumGenre{}).
		Joins("JOIN albums ON albums.id=album_genres.album_id").
		Joins("LEFT JOIN tracks ON tracks.album_id=albums.id").
		Group("album_genres.genre_id").
		Having("count(tracks.id)=0").
		SubQuery()
	q = s.db.
		Where("genres.id IN ?", subAlbumGenresNoTracks).
		Delete(db.Genre{})
	st.genresMissing += int(q.RowsAffected)

	return nil
}

func (s *Scanner) cleanBookmarks(st *State) error {
	start := time.Now()
	defer func() {
		log.Printf("finished clean bookmarks in %s, %d removed", durSince(start), st.BookmarksRemoved())
	}()

	trackBookmarks := s.db.
		Select("bookmarks.id").
		Model(db.Bookmark{}).
		Joins("LEFT JOIN tracks ON tracks.id=bookmarks.entry_id").
		Where("tracks.id IS NULL AND bookmarks.entry_id_type=?", specid.Track).
		SubQuery()
	q := s.db.
		Where("bookmarks.id IN ?", trackBookmarks).
		Delete(db.Bookmark{})
	st.bookmarksRemoved += int(q.RowsAffected)

	podcastBookmarks := s.db.
		Select("bookmarks.id").
		Model(db.Bookmark{}).
		Joins("LEFT JOIN podcast_episodes ON podcast_episodes.id=bookmarks.entry_id").
		Where("podcast_episodes.id IS NULL AND bookmarks.entry_id_type=?", specid.PodcastEpisode).
		SubQuery()
	q = s.db.
		Where("bookmarks.id IN ?", podcastBookmarks).
		Delete(db.Bookmark{})
	st.bookmarksRemoved += int(q.RowsAffected)

	return nil
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

type State struct {
	errs   []error
	isFull bool

	seenTracks    map[int]struct{}
	seenAlbums    map[int]struct{}
	seenTracksNew int

	tracksMissing    []int64
	albumsMissing    []int64
	artistsMissing   int
	genresMissing    int
	bookmarksRemoved int
}

func (s *State) SeenTracks() int    { return len(s.seenTracks) }
func (s *State) SeenAlbums() int    { return len(s.seenAlbums) }
func (s *State) SeenTracksNew() int { return s.seenTracksNew }

func (s *State) TracksMissing() int    { return len(s.tracksMissing) }
func (s *State) AlbumsMissing() int    { return len(s.albumsMissing) }
func (s *State) ArtistsMissing() int   { return s.artistsMissing }
func (s *State) GenresMissing() int    { return s.genresMissing }
func (s *State) BookmarksRemoved() int { return s.bookmarksRemoved }

type MultiValueMode uint8

const (
	None MultiValueMode = iota
	Delim
	Multi
)

type Tag uint8

const (
	Genre Tag = iota
	Artist
	AlbumArtist
)

type MultiValueSetting struct {
	Mode  MultiValueMode
	Delim string
}

func ParseMulti(setting MultiValueSetting, values []string, value string) []string {
	var parts []string
	switch setting.Mode {
	case Multi:
		parts = values
	case Delim:
		parts = strings.Split(value, setting.Delim)
	default:
		parts = []string{value}
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	parts = slices.DeleteFunc(parts, func(s string) bool {
		return s == ""
	})
	return parts
}

func musicDirRelative(musicDirs []string, absPath string) (musicDir, relPath string) {
	for _, musicDir := range musicDirs {
		if fileutil.HasPrefix(absPath, musicDir) {
			relPath, _ = filepath.Rel(musicDir, absPath)
			return musicDir, relPath
		}
	}
	return
}

func symWalk(absPath string, fn fs.WalkDirFunc) error {
	eval, _ := filepath.EvalSymlinks(absPath)
	return filepath.WalkDir(eval, func(subAbs string, d fs.DirEntry, err error) error {
		subAbs = strings.Replace(subAbs, eval, absPath, 1)
		return fn(subAbs, d, err)
	})
}
