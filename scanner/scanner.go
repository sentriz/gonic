//nolint:nestif,goconst
package scanner

import (
	"cmp"
	"context"
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

	"github.com/djherbis/times"
	"github.com/fsnotify/fsnotify"
	"github.com/jinzhu/gorm"
	"github.com/rainycape/unidecode"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/fileutil"
	"go.senan.xyz/gonic/tags"
	"go.senan.xyz/wrtag/coverparse"
	"go.senan.xyz/wrtag/tags/normtag"
)

var (
	ErrAlreadyScanning = errors.New("already scanning")
	ErrReadingTags     = errors.New("could not read tags")
	albumSortBuf       = &collate.Buffer{}
	albumSortCollator  = collate.New(language.English)
)

type Scanner struct {
	db                 *db.DB
	musicDirs          []string
	multiValueSettings map[*tags.Spec]tags.MultiValueSetting
	tagReader          tags.Reader
	excludePattern     *regexp.Regexp
	scanEmbeddedCover  bool
	genreTree          map[string][]string
	scanning           *int32
}

func New(musicDirs []string, db *db.DB, multiValueSettings map[*tags.Spec]tags.MultiValueSetting, tagReader tags.Reader, excludePattern string, scanEmbeddedCover bool, genreTree map[string][]string) *Scanner {
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
		scanEmbeddedCover:  scanEmbeddedCover,
		genreTree:          genreTree,
		scanning:           new(int32),
	}
}

func (s *Scanner) IsScanning() bool    { return atomic.LoadInt32(s.scanning) == 1 }
func (s *Scanner) StartScanning() bool { return atomic.CompareAndSwapInt32(s.scanning, 0, 1) }
func (s *Scanner) StopScanning()       { atomic.StoreInt32(s.scanning, 0) }

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
	if err := s.cleanAlbumMetadata(); err != nil {
		return nil, fmt.Errorf("clean album metadata: %w", err)
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
				batchT.Reset(batchInterval)
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

		// skip macOS ._ resource fork files
		if strings.HasPrefix(item.Name(), "._") {
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

	// read tags outside the transaction to avoid holding db locks during disk i/o
	type trackTagData struct {
		trackUpdate
		trprops tags.Properties
		trags   tags.Tags
	}
	tagData := make([]trackTagData, 0, len(trackUpdates))
	for _, t := range trackUpdates {
		trprops, trags, err := s.tagReader.Read(t.absPath)
		if err != nil {
			return fmt.Errorf("read %q: %w: %w", t.basename, err, ErrReadingTags)
		}
		tagData = append(tagData, trackTagData{trackUpdate: t, trprops: trprops, trags: trags})
	}

	return s.db.Transaction(func(tx *db.DB) error {
		var discTitles = map[int]string{}
		for _, t := range tagData {
			if err := s.populateTrackAndArtists(tx, st, t.i, &album, t.track, t.timeSpec, t.trprops, t.trags, t.basename, t.absPath); err != nil {
				return fmt.Errorf("populate track %q: %w", t.basename, err)
			}

			discNum := cmp.Or(tags.ParseInt(normtag.Get(t.trags, normtag.DiscNumber)), 1)
			discSubtitle := normtag.Get(t.trags, "DISCSUBTITLE")

			if _, exists := discTitles[discNum]; !exists && discSubtitle != "" {
				discTitles[discNum] = discSubtitle
			}
		}

		if err := populateAlbumDiscTitles(tx, &album, discTitles); err != nil {
			return fmt.Errorf("populate disc titles: %w", err)
		}
		return nil
	})
}

//nolint:gocyclo
func (s *Scanner) populateTrackAndArtists(tx *db.DB, st *State, i int, album *db.Album, track *db.Track, timeSpec times.Timespec, trprops tags.Properties, trags tags.Tags, basename, absPath string) error {
	genreNames := tags.ReadValues(trags, tags.Genre, s.multiValueSettings)
	genreIDs, err := populateGenres(tx, genreNames)
	if err != nil {
		return fmt.Errorf("populate genres: %w", err)
	}

	var inheritedGenreIDs []int
	if len(s.genreTree) > 0 {
		direct := map[string]struct{}{}
		for _, name := range genreNames {
			direct[name] = struct{}{}
		}
		var inheritedNames []string
		for parent, descendants := range s.genreTree {
			if _, ok := direct[parent]; ok {
				continue
			}
			for _, desc := range descendants {
				if _, ok := direct[desc]; ok {
					inheritedNames = append(inheritedNames, parent)
					break
				}
			}
		}
		inheritedGenreIDs, err = populateGenres(tx, inheritedNames)
		if err != nil {
			return fmt.Errorf("populate inherited genres: %w", err)
		}
	}

	modTime, createTime := timeSpec.ModTime(), timeSpec.ModTime()
	if timeSpec.HasBirthTime() {
		createTime = timeSpec.BirthTime()
	}

	albumArtistEntries := tags.ReadCredits(trags, tags.AlbumArtist, s.multiValueSettings)
	trackArtistEntries := tags.ReadCredits(trags, tags.Artist, s.multiValueSettings)

	contributorEntries := make([][]tags.Credited, 0, len(trackContributorRoles))
	for _, r := range trackContributorRoles {
		contributorEntries = append(contributorEntries, tags.ReadCredits(trags, r.spec, nil))
	}

	// if the same artist name appears with an MBID anywhere in this track's tags, prefer that MBID for the same name on credits where the
	// MBID is absent. lets a remixer/composer with just a name attach to the right DB row when multiple rows share the name
	artistMusicBrainzID := map[string]string{}
	for _, group := range append([][]tags.Credited{albumArtistEntries, trackArtistEntries}, contributorEntries...) {
		for _, e := range group {
			if e.MusicBrainzID != "" && artistMusicBrainzID[e.Value] == "" {
				artistMusicBrainzID[e.Value] = e.MusicBrainzID
			}
		}
	}

	// metadata for the album table comes only from the first track's tags
	if i == 0 {
		if err := tx.Where("album_id=?", album.ID).Delete(db.AlbumCredit{}).Error; err != nil {
			return fmt.Errorf("delete album credits: %w", err)
		}

		var albumArtists []artistRef
		for _, e := range albumArtistEntries {
			artist, err := populateArtist(tx, e.Value, cmp.Or(e.MusicBrainzID, artistMusicBrainzID[e.Value]))
			if err != nil {
				return fmt.Errorf("populate album artist: %w", err)
			}
			albumArtists = append(albumArtists, artistRef{id: artist.ID, credit: e.ValueCredit})
		}
		if err := populateAlbumCredits(tx, album, db.RoleAlbumArtist, albumArtists); err != nil {
			return fmt.Errorf("populate album credits: %w", err)
		}

		if err := populateAlbum(tx, album, trags, modTime, createTime); err != nil {
			return fmt.Errorf("populate album: %w", err)
		}

		if err := populateAlbumGenres(tx, album, genreIDs, inheritedGenreIDs); err != nil {
			return fmt.Errorf("populate album genres: %w", err)
		}

		if err := populateAlbumLabels(tx, album, normtag.Values(trags, normtag.Label)); err != nil {
			return fmt.Errorf("populate album labels: %w", err)
		}
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stating %q: %w", basename, err)
	}

	if track == nil {
		track = &db.Track{}
	}

	if err := populateTrack(tx, s.scanEmbeddedCover, album, track, trprops, trags, basename, int(stat.Size()), createTime); err != nil {
		return fmt.Errorf("process %q: %w", basename, err)
	}
	if err := populateTrackGenres(tx, track, genreIDs, inheritedGenreIDs); err != nil {
		return fmt.Errorf("populate track genres: %w", err)
	}

	isrcs := tags.ReadValues(trags, tags.ISRC, s.multiValueSettings)
	if err := populateTrackISRCs(tx, track, isrcs); err != nil {
		return fmt.Errorf("populate track ISRCs: %w", err)
	}

	if err := tx.Where("track_id=?", track.ID).Delete(db.TrackCredit{}).Error; err != nil {
		return fmt.Errorf("delete track credits: %w", err)
	}

	var trackArtists []artistRef
	for _, e := range trackArtistEntries {
		artist, err := populateArtist(tx, e.Value, cmp.Or(e.MusicBrainzID, artistMusicBrainzID[e.Value]))
		if err != nil {
			return fmt.Errorf("populate track artist: %w", err)
		}
		trackArtists = append(trackArtists, artistRef{id: artist.ID, credit: e.ValueCredit})
	}
	if err := populateTrackCredits(tx, track, db.RoleArtist, trackArtists); err != nil {
		return fmt.Errorf("populate track credits: %w", err)
	}

	var contributorRows [][]any
	for ci, r := range trackContributorRoles {
		for _, e := range contributorEntries[ci] {
			artist, err := populateArtist(tx, e.Value, cmp.Or(e.MusicBrainzID, artistMusicBrainzID[e.Value]))
			if err != nil {
				return fmt.Errorf("populate contributor artist: %w", err)
			}
			contributorRows = append(contributorRows, []any{artist.ID, r.role, e.ValueCredit})
		}
	}
	if err := tx.InsertBulkLeftManyRows("track_credits", []string{"track_id", "artist_id", "role", "credited_as"}, track.ID, contributorRows); err != nil {
		return fmt.Errorf("insert bulk track contributor credits: %w", err)
	}

	// possible album level embedded covers come only from the first track
	if i == 0 {
		if err := populateAlbumEmbeddedCover(tx, s.scanEmbeddedCover, album, track, trprops); err != nil {
			return fmt.Errorf("populate embedded cover: %w", err)
		}
	}

	st.seenTracks[track.ID] = struct{}{}
	st.seenTracksNew++

	return nil
}

func populateAlbumEmbeddedCover(tx *db.DB, scanEmbeddedCover bool, album *db.Album, track *db.Track, trprops tags.Properties) error {
	var trackID int
	if scanEmbeddedCover && trprops.HasCover {
		trackID = track.ID
	}
	var prevTrackID int
	if album.EmbeddedCoverTrackID != nil {
		prevTrackID = *album.EmbeddedCoverTrackID
	}
	if prevTrackID == trackID {
		return nil
	}

	album.EmbeddedCoverTrackID = nil
	if trackID > 0 {
		album.EmbeddedCoverTrackID = &trackID
	}
	if err := tx.Save(album).Error; err != nil {
		return fmt.Errorf("saving album for embedded cover track id: %w", err)
	}
	return nil
}

func populateAlbum(tx *db.DB, album *db.Album, trags map[string][]string, modTime, createTime time.Time) error {
	album.TagTitle, _ = tags.Read(trags, tags.AlbumTitle)
	album.TagTitleUDec = decoded(album.TagTitle)
	album.TagAlbumArtist, album.TagAlbumArtistCredit = tags.Read(trags, tags.AlbumArtist)
	album.TagBrainzID = normtag.Get(trags, normtag.MusicBrainzReleaseID)
	album.TagYear = 0
	if v, _ := tags.Read(trags, tags.Year); v != "" {
		album.TagYear = tags.ParseDate(v).Year()
	}
	album.TagCompilation = tags.ParseBool(normtag.Get(trags, normtag.Compilation))
	album.TagReleaseType = strings.Join(normtag.Values(trags, normtag.ReleaseType), ", ")

	album.ModifiedAt = modTime
	if album.CreatedAt.After(createTime) {
		album.CreatedAt = createTime // reset created at to match filesytem for new albums
	}

	if err := tx.Save(album).Error; err != nil {
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
	album.RightPathSortKey = string(albumSortCollator.KeyFromString(albumSortBuf, basename))
	album.ParentID = parent.ID

	if err := tx.Save(album).Error; err != nil {
		return fmt.Errorf("saving album: %w", err)
	}

	return nil
}

func populateTrack(tx *db.DB, scanEmbeddedCover bool, album *db.Album, track *db.Track, trprops tags.Properties, trags map[string][]string, basename string, size int, createTime time.Time) error {
	track.Filename = basename
	track.FilenameUDec = decoded(basename)
	track.Size = size
	track.AlbumID = album.ID
	track.TagLyrics = normtag.Get(trags, normtag.Lyrics)

	if track.CreatedAt.IsZero() || track.CreatedAt.After(createTime) {
		track.CreatedAt = createTime
	}

	track.TagTitle, _ = tags.Read(trags, tags.TrackTitle)
	track.TagTitleUDec = decoded(track.TagTitle)
	track.TagTrackArtist, track.TagTrackArtistCredit = tags.Read(trags, tags.Artist)
	track.TagTrackNumber = tags.ParseInt(normtag.Get(trags, normtag.TrackNumber))
	track.TagDiscNumber = tags.ParseInt(normtag.Get(trags, normtag.DiscNumber))
	track.TagBrainzID = normtag.Get(trags, normtag.MusicBrainzRecordingID)
	track.TagYear = 0
	if v, _ := tags.Read(trags, tags.Year); v != "" {
		track.TagYear = tags.ParseDate(v).Year()
	}

	track.ReplayGainTrackGain = tags.ParseDB(normtag.Get(trags, normtag.ReplayGainTrackGain))
	track.ReplayGainTrackPeak = tags.ParseFloat(normtag.Get(trags, normtag.ReplayGainTrackPeak))
	track.ReplayGainAlbumGain = tags.ParseDB(normtag.Get(trags, normtag.ReplayGainAlbumGain))
	track.ReplayGainAlbumPeak = tags.ParseFloat(normtag.Get(trags, normtag.ReplayGainAlbumPeak))

	track.HasEmbeddedCover = false
	if scanEmbeddedCover {
		track.HasEmbeddedCover = trprops.HasCover
	}

	// these two are calculated from the file instead of tags
	track.Length = int(trprops.Length.Seconds())
	track.Bitrate = int(trprops.Bitrate)

	if err := tx.Save(track).Error; err != nil {
		return fmt.Errorf("saving track: %w", err)
	}

	return nil
}

func populateArtist(tx *db.DB, artistName, musicBrainzID string) (*db.Artist, error) {
	nameUDec := decoded(artistName)

	if musicBrainzID == "" {
		// no MBID, use any
		var artist db.Artist
		return &artist, tx.
			Where(db.Artist{Name: artistName}).
			Order("id ASC").
			Attrs(db.Artist{NameUDec: nameUDec}).
			FirstOrCreate(&artist).
			Error
	}

	// have MBID

	var artist db.Artist
	err := tx.
		Where(db.Artist{MusicBrainzID: musicBrainzID}).
		First(&artist).
		Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if artist.ID != 0 {
		return &artist, nil
	}

	// adopt the name only row if one exists, else create
	return &artist, tx.
		Where(db.Artist{Name: artistName}).
		Where("music_brainz_id = ''").
		Attrs(db.Artist{NameUDec: nameUDec}).
		Assign(db.Artist{MusicBrainzID: musicBrainzID}).
		FirstOrCreate(&artist).Error
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

func populateTrackGenres(tx *db.DB, track *db.Track, directIDs, inheritedIDs []int) error {
	if err := tx.Where("track_id=?", track.ID).Delete(db.TrackGenre{}).Error; err != nil {
		return fmt.Errorf("delete old track genre records: %w", err)
	}
	rows := genreRows(directIDs, inheritedIDs)
	if err := tx.InsertBulkLeftManyRows("track_genres", []string{"track_id", "genre_id", "inherited"}, track.ID, rows); err != nil {
		return fmt.Errorf("insert track genres: %w", err)
	}
	return nil
}

func populateTrackISRCs(tx *db.DB, track *db.Track, isrcs []string) error {
	if err := tx.Where("track_id=?", track.ID).Delete(db.TrackISRC{}).Error; err != nil {
		return fmt.Errorf("delete old track ISRCs records: %w", err)
	}

	var col [][]any
	for _, isrc := range isrcs {
		if isrc == "" {
			continue
		}
		col = append(col, []any{isrc})
	}
	if err := tx.InsertBulkLeftManyRows("track_isrcs", []string{"track_id", "isrc"}, track.ID, col); err != nil {
		return fmt.Errorf("insert bulk track ISRCs: %w", err)
	}
	return nil
}

func populateAlbumLabels(tx *db.DB, album *db.Album, labels []string) error {
	if err := tx.Where("album_id=?", album.ID).Delete(db.AlbumLabel{}).Error; err != nil {
		return fmt.Errorf("delete old album labels: %w", err)
	}

	seen := map[string]struct{}{}
	var rows [][]any
	for _, l := range labels {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if _, ok := seen[l]; ok {
			continue
		}
		seen[l] = struct{}{}
		rows = append(rows, []any{l})
	}
	if err := tx.InsertBulkLeftManyRows("album_labels", []string{"album_id", "label"}, album.ID, rows); err != nil {
		return fmt.Errorf("insert bulk album labels: %w", err)
	}
	return nil
}

func populateAlbumGenres(tx *db.DB, album *db.Album, directIDs, inheritedIDs []int) error {
	if err := tx.Where("album_id=?", album.ID).Delete(db.AlbumGenre{}).Error; err != nil {
		return fmt.Errorf("delete old album genre records: %w", err)
	}
	rows := genreRows(directIDs, inheritedIDs)
	if err := tx.InsertBulkLeftManyRows("album_genres", []string{"album_id", "genre_id", "inherited"}, album.ID, rows); err != nil {
		return fmt.Errorf("insert album genres: %w", err)
	}
	return nil
}

func genreRows(directIDs, inheritedIDs []int) [][]any {
	seen := map[int]struct{}{}
	var rows [][]any
	for _, id := range directIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		rows = append(rows, []any{id, false})
	}
	for _, id := range inheritedIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		rows = append(rows, []any{id, true})
	}
	return rows
}

func populateAlbumDiscTitles(tx *db.DB, album *db.Album, discTitles map[int]string) error {
	if err := tx.Where("album_id=?", album.ID).Delete(db.AlbumDiscTitle{}).Error; err != nil {
		return fmt.Errorf("delete old album disc titles: %w", err)
	}

	for discNum, title := range discTitles {
		discTitle := db.AlbumDiscTitle{
			AlbumID:    album.ID,
			DiscNumber: discNum,
			Title:      title,
		}
		if err := tx.Create(&discTitle).Error; err != nil {
			return fmt.Errorf("create disc title: %w", err)
		}
	}
	return nil
}

func populateAlbumCredits(tx *db.DB, album *db.Album, role string, refs []artistRef) error {
	rows := make([][]any, len(refs))
	for i, r := range refs {
		rows[i] = []any{r.id, role, r.credit}
	}
	if err := tx.InsertBulkLeftManyRows("album_credits", []string{"album_id", "artist_id", "role", "credited_as"}, album.ID, rows); err != nil {
		return fmt.Errorf("insert bulk album credits: %w", err)
	}
	return nil
}

func populateTrackCredits(tx *db.DB, track *db.Track, role string, refs []artistRef) error {
	rows := make([][]any, len(refs))
	for i, r := range refs {
		rows[i] = []any{r.id, role, r.credit}
	}
	if err := tx.InsertBulkLeftManyRows("track_credits", []string{"track_id", "artist_id", "role", "credited_as"}, track.ID, rows); err != nil {
		return fmt.Errorf("insert bulk track credits: %w", err)
	}
	return nil
}

type artistRef struct {
	id     int
	credit string
}

//nolint:gochecknoglobals
var trackContributorRoles = []struct {
	role string
	spec *tags.Spec
}{
	{db.RoleRemixer, tags.Remixer},
	{db.RoleComposer, tags.Composer},
	{db.RoleLyricist, tags.Lyricist},
	{db.RoleConductor, tags.Conductor},
	{db.RoleProducer, tags.Producer},
	{db.RoleArranger, tags.Arranger},
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

func (s *Scanner) cleanAlbumMetadata() error {
	var numModified int

	start := time.Now()
	defer func() { log.Printf("finished clean album metadata in %s, %d modified", durSince(start), numModified) }()

	subTracks := s.db.Model(db.Track{}).Select("DISTINCT album_id").SubQuery()

	var emptyAlbumIDs []int
	err := s.db.
		Model(db.Album{}).
		Where("id NOT IN ?", subTracks).
		Where("tag_title != '' OR tag_album_artist != ''").
		Pluck("id", &emptyAlbumIDs).
		Error
	if err != nil {
		return fmt.Errorf("finding empty albums: %w", err)
	}
	if len(emptyAlbumIDs) == 0 {
		return nil
	}

	numModified = len(emptyAlbumIDs)

	if err := s.db.Where("album_id IN (?)", emptyAlbumIDs).Delete(db.AlbumCredit{}).Error; err != nil {
		return err
	}
	if err := s.db.Where("album_id IN (?)", emptyAlbumIDs).Delete(db.AlbumGenre{}).Error; err != nil {
		return err
	}
	if err := s.db.Where("album_id IN (?)", emptyAlbumIDs).Delete(db.AlbumDiscTitle{}).Error; err != nil {
		return err
	}
	if err := s.db.Where("album_id IN (?)", emptyAlbumIDs).Delete(db.AlbumLabel{}).Error; err != nil {
		return err
	}

	q := s.db.
		Model(&db.Album{}).
		Where("id IN (?)", emptyAlbumIDs).
		Updates(map[string]any{
			"tag_title":               "",
			"tag_title_u_dec":         "",
			"tag_album_artist":        "",
			"tag_year":                0,
			"tag_brainz_id":           "",
			"tag_compilation":         0,
			"tag_release_type":        "",
			"embedded_cover_track_id": nil,
		})
	if err := q.Error; err != nil {
		return err
	}

	numModified = int(q.RowsAffected)

	return nil
}

func (s *Scanner) cleanArtists(st *State) error {
	start := time.Now()
	defer func() { log.Printf("finished clean artists in %s, %d removed", durSince(start), st.ArtistsMissing()) }()

	// gorm doesn't seem to support subqueries without parens for UNION
	q := s.db.Exec(`
		DELETE FROM artists
		WHERE id NOT IN (
			SELECT artist_id FROM album_credits
			UNION
			SELECT artist_id FROM track_credits
		)
    `)
	if err := q.Error; err != nil {
		return err
	}
	st.artistsMissing = int(q.RowsAffected)
	return nil
}

func (s *Scanner) cleanGenres(st *State) error {
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
	if err := q.Error; err != nil {
		return fmt.Errorf("delete unused genres: %w", err)
	}
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
	if err := q.Error; err != nil {
		return fmt.Errorf("delete album-only genres without tracks: %w", err)
	}
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
		Where("tracks.id IS NULL AND bookmarks.entry_id_type=?", db.BookmarkEntryTrack).
		SubQuery()
	q := s.db.
		Where("bookmarks.id IN ?", trackBookmarks).
		Delete(db.Bookmark{})
	if err := q.Error; err != nil {
		return fmt.Errorf("delete orphaned track bookmarks: %w", err)
	}
	st.bookmarksRemoved += int(q.RowsAffected)

	podcastBookmarks := s.db.
		Select("bookmarks.id").
		Model(db.Bookmark{}).
		Joins("LEFT JOIN podcast_episodes ON podcast_episodes.id=bookmarks.entry_id").
		Where("podcast_episodes.id IS NULL AND bookmarks.entry_id_type=?", db.BookmarkEntryPodcastEpisode).
		SubQuery()
	q = s.db.
		Where("bookmarks.id IN ?", podcastBookmarks).
		Delete(db.Bookmark{})
	if err := q.Error; err != nil {
		return fmt.Errorf("delete orphaned podcast bookmarks: %w", err)
	}
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
