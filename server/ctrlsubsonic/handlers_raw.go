package ctrlsubsonic

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/djherbis/times"
	"github.com/jinzhu/gorm"
	"go.senan.xyz/wrtag/coverparse"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/fileutil"
	"go.senan.xyz/gonic/infocache/artistinfocache"
	"go.senan.xyz/gonic/playlist"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specidpaths"
	"go.senan.xyz/gonic/tags"
	"go.senan.xyz/gonic/transcode"
)

// "raw" handlers are ones that don't always return a spec response.
// it could be a file, stream, etc. so you must either
//   a) write to response writer
//   b) return a non-nil spec.Response
//  _but not both_

const (
	coverDefaultSize = 600
)

var (
	//nolint:gochecknoglobals
	// after the image is decoded with image.Decode, we use the "format" param as the extension
	coverCacheFormats = []string{
		"jpeg", // from image.RegisterFormat("jpeg") @ image/jpeg/reader.go
		"png",  // from image.RegisterFormat("png")  @ image/png/reader.go
		"gif",  // from image.RegisterFormat("gif")  @ image/gif/reader.go
		"bmp",  // from image.RegisterFormat("bmp")  @ golang.org/x/image/bmp/reader.go
		"tiff", // from image.RegisterFormat("tiff") @ golang.org/x/image/tiff/reader.go
	}
)

func (c *Controller) ServeGetCoverArt(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}

	size := params.GetOrInt("size", coverDefaultSize)
	if size <= 0 {
		return spec.NewError(0, "invalid size")
	}

	if id.Type == specid.Playlist {
		user := r.Context().Value(CtxUser).(*db.User)
		pl, err := c.playlistStore.Read(playlistIDDecode(id))
		if err != nil {
			return spec.NewError(70, "playlist with id %s not found", id)
		}
		if pl.UserID != user.ID && !pl.IsPublic {
			return spec.NewError(50, "you aren't allowed to read that user's playlist")
		}
	}

	// Resolve local artwork before locking the cache: the source may be on a slow mount.
	source, err := c.coverSourceFor(id)
	if err != nil {
		return spec.NewError(70, "couldn't find cover %q: %v", id, err)
	}
	defer source.close()

	c.coverCache.RLock()
	defer c.coverCache.RUnlock()

	// cached cover could exist for any supported format
	cachePath, err := findCachedCover(c.coverCache.Path(), source.cacheID, size)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("error checking cache: %v", err)
		return nil
	}

	serve := func(p string) {
		source.close() // Do not retain the local source while a slow client reads the cached image.
		w.Header().Set("Cache-Control", "public, max-age=1209600")
		http.ServeFile(w, r, p) //nolint:gosec // p is contained to the cover cache dir via fileutil.SafeJoin
	}

	if cachePath != "" {
		_ = os.Chtimes(cachePath, time.Now(), time.Now()) // touch for LRU eviction
		serve(cachePath)
		return nil
	}

	reader, err := source.reader(c, id)
	if err != nil {
		return spec.NewError(70, "couldn't find cover %q: %v", id, err)
	}
	defer reader.Close()

	img, format, err := image.Decode(reader)
	if err != nil {
		return spec.NewError(0, "decode for cover %q: %v", id, err)
	}
	if err := source.checkUnchanged(); err != nil {
		return spec.NewError(70, "source changed while reading cover %q: %v", id, err)
	}

	// don't upscale
	minSize := min(size, max(img.Bounds().Dx(), img.Bounds().Dy()))

	cachePath, err = fileutil.SafeJoin(c.coverCache.Path(), coverCacheFilename(source.cacheID, minSize, format))
	if err != nil {
		return spec.NewError(0, "bad cover id %q: %v", id, err)
	}

	if minSize != size {
		// we down sized, check cache again
		if _, err := os.Stat(cachePath); err == nil {
			_ = os.Chtimes(cachePath, time.Now(), time.Now()) // touch for LRU eviction
			serve(cachePath)
			return nil
		}
	}

	resized := imaging.Fit(img, minSize, minSize, imaging.Lanczos)

	if err := imaging.Save(resized, cachePath); err != nil {
		return spec.NewError(0, "saving cover %q: %v", id, err)
	}

	serve(cachePath)
	return nil
}

type coverSource struct {
	cacheID string
	file    *os.File
	info    os.FileInfo
}

func (source *coverSource) close() {
	if source.file != nil {
		_ = source.file.Close()
		source.file = nil
	}
}

func (c *Controller) coverSourceFor(id specid.ID) (*coverSource, error) {
	source := &coverSource{cacheID: id.String()}
	if id.Type == specid.Artist {
		return source, nil // Remote artist images retain their existing cache policy.
	}
	var err error
	var revision int64
	switch id.Type {
	case specid.Track:
		var track db.Track
		err = c.dbc.Select("id, album_id, filename, updated_at").
			Preload("Album", func(query *gorm.DB) *gorm.DB {
				return query.Select("id, root_dir, left_path, right_path")
			}).First(&track, id.Value).Error
		if err != nil {
			return nil, fmt.Errorf("select track: %w", err)
		}
		source.file, err = os.Open(track.AbsPath())
		if !track.UpdatedAt.IsZero() {
			revision = track.UpdatedAt.UnixNano()
		}
	default:
		var reader io.ReadCloser
		reader, err = coverFor(c.dbc, c.artistInfoCache, c.playlistStore, c.tagReader, id)
		if err == nil {
			var ok bool
			source.file, ok = reader.(*os.File)
			if !ok {
				reader.Close()
				return nil, errCoverNotFound
			}
		}
	}
	if err != nil {
		return nil, err
	}
	source.info, err = source.file.Stat()
	if err != nil {
		source.file.Close()
		return nil, err
	}
	// Hash the entire identity so even long playlist IDs produce short filenames.
	key := fmt.Sprintf("%s\x00%s\x00%s\x00%d", id, source.file.Name(), coverFileVersion(source.info), revision)
	source.cacheID = fmt.Sprintf("local-%x", sha256.Sum256([]byte(key)))
	return source, nil
}

func coverFileVersion(info os.FileInfo) string {
	stamp := times.Get(info)
	var changed int64
	if stamp.HasChangeTime() {
		changed = stamp.ChangeTime().UnixNano()
	}
	return fmt.Sprintf("%d:%d:%d", info.Size(), info.ModTime().UnixNano(), changed)
}

func (source *coverSource) reader(c *Controller, id specid.ID) (io.ReadCloser, error) {
	if source.file == nil {
		return coverFor(c.dbc, c.artistInfoCache, c.playlistStore, c.tagReader, id)
	}
	if id.Type != specid.Track {
		// Keep the selected file open: a playlist's preferred cover can change meanwhile.
		return io.NopCloser(source.file), nil
	}
	cover, err := c.tagReader.ReadCover(source.file.Name())
	if err != nil {
		return nil, fmt.Errorf("read cover: %w", err)
	}
	if len(cover) == 0 {
		return nil, errCoverEmpty
	}
	return io.NopCloser(bytes.NewReader(cover)), nil
}

func (source *coverSource) checkUnchanged() error {
	if source.file == nil {
		return nil
	}
	info, err := os.Stat(source.file.Name()) //nolint:gosec // same DB-backed or SafeJoin-validated path opened by coverSourceFor
	if err != nil {
		return err
	}
	if !os.SameFile(source.info, info) || coverFileVersion(source.info) != coverFileVersion(info) {
		return errors.New("cover source was replaced or modified")
	}
	return nil
}

func coverCacheFilename(idStr string, size int, format string) string {
	return fmt.Sprintf("%s-%d.%s", idStr, size, format)
}

func findCachedCover(cacheDir, idStr string, size int) (string, error) {
	for _, format := range coverCacheFormats {
		cachePath, err := fileutil.SafeJoin(cacheDir, coverCacheFilename(idStr, size, format))
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(cachePath); err == nil {
			return cachePath, nil
		}
	}
	return "", os.ErrNotExist
}

var (
	errCoverNotFound = errors.New("could not find a cover with that id")
	errCoverEmpty    = errors.New("no cover found")
)

// TODO: can we use specidpaths.Locate here?
func coverFor(dbc *db.DB, artistInfoCache *artistinfocache.ArtistInfoCache, playlistStore *playlist.Store, tagReader tags.Reader, id specid.ID) (io.ReadCloser, error) {
	switch id.Type {
	case specid.Album:
		return coverForAlbum(dbc, id.Value)
	case specid.Artist:
		return coverForArtist(artistInfoCache, id.Value)
	case specid.Podcast:
		return coverForPodcast(dbc, id.Value)
	case specid.PodcastEpisode:
		return coverForPodcastEpisode(dbc, id.Value)
	case specid.Track:
		return coverForTrack(dbc, tagReader, id.Value)
	case specid.Playlist:
		return coverForPlaylist(playlistStore, id)
	default:
		return nil, errCoverNotFound
	}
}

func coverForAlbum(dbc *db.DB, id int) (*os.File, error) {
	var folder db.Album
	err := dbc.DB.
		Select("id, root_dir, left_path, right_path, cover").
		First(&folder, id).
		Error
	if err != nil {
		return nil, fmt.Errorf("select album: %w", err)
	}
	if folder.Cover == "" {
		return nil, errCoverEmpty
	}
	return os.Open(filepath.Join(folder.RootDir, folder.LeftPath, folder.RightPath, folder.Cover))
}

func coverForArtist(artistInfoCache *artistinfocache.ArtistInfoCache, id int) (io.ReadCloser, error) {
	info, err := artistInfoCache.Get(context.Background(), id)
	if err != nil {
		return nil, fmt.Errorf("get artist info from cache: %w", err)
	}
	if info.ImageURL == "" {
		return nil, fmt.Errorf("%w: cache miss", errCoverEmpty)
	}
	resp, err := http.Get(info.ImageURL) //nolint:gosec // url comes from trusted lastfm artist info cache
	if err != nil {
		return nil, fmt.Errorf("req image from lastfm: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("req image from lastfm: bad status %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func coverForPodcast(dbc *db.DB, id int) (*os.File, error) {
	var podcast db.Podcast
	err := dbc.
		First(&podcast, id).
		Error
	if err != nil {
		return nil, fmt.Errorf("select podcast: %w", err)
	}
	if podcast.Image == "" {
		return nil, errCoverEmpty
	}
	return os.Open(filepath.Join(podcast.RootDir, podcast.Image))
}

func coverForPodcastEpisode(dbc *db.DB, id int) (*os.File, error) {
	var pe db.PodcastEpisode
	err := dbc.
		Preload("Podcast").
		First(&pe, id).
		Error
	if err != nil {
		return nil, fmt.Errorf("select episode: %w", err)
	}
	if pe.Podcast == nil || pe.Podcast.Image == "" {
		return nil, errCoverEmpty
	}
	return os.Open(filepath.Join(pe.Podcast.RootDir, pe.Podcast.Image))
}

func coverForTrack(dbc *db.DB, tagReader tags.Reader, id int) (io.ReadCloser, error) {
	var tr db.Track
	err := dbc.
		Preload("Album").
		First(&tr, id).
		Error
	if err != nil {
		return nil, fmt.Errorf("select track: %w", err)
	}

	absPath := tr.AbsPath()

	cover, err := tagReader.ReadCover(absPath)
	if err != nil {
		return nil, fmt.Errorf("read cover: %w", err)
	}
	if len(cover) == 0 {
		return nil, errCoverEmpty
	}

	return io.NopCloser(bytes.NewReader(cover)), nil
}

func coverForPlaylist(playlistStore *playlist.Store, id specid.ID) (*os.File, error) {
	playlistPath := playlistIDDecode(id)
	playlistDir, err := fileutil.SafeJoin(playlistStore.BasePath(), filepath.Dir(playlistPath))
	if err != nil {
		return nil, err
	}
	playlistBase := strings.TrimSuffix(filepath.Base(playlistPath), filepath.Ext(playlistPath))

	entries, err := os.ReadDir(playlistDir)
	if err != nil {
		return nil, fmt.Errorf("read playlist dir: %w", err)
	}

	var cover string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		nameBase := strings.TrimSuffix(name, filepath.Ext(name))
		if nameBase == playlistBase && coverparse.IsCover(name) {
			cover = coverparse.BestBetween(cover, name)
		}
	}

	if cover == "" {
		return nil, errCoverEmpty
	}

	return os.Open(filepath.Join(playlistDir, cover)) //nolint:gosec // playlistDir validated via SafeJoin above
}

func (c *Controller) ServeStream(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	urlPath := r.URL.Path
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}

	file, err := specidpaths.Locate(c.dbc, id)
	if err != nil {
		return spec.NewError(0, "error looking up id %s: %v", id, err)
	}

	audioFile, ok := file.(db.AudioFile)
	if !ok {
		return spec.NewError(0, "type of id does not contain audio")
	}

	maxBitRate, _ := params.GetInt("maxBitRate")
	format, _ := params.Get("format")
	timeOffset, _ := params.GetInt("timeOffset")
	estimateLength := params.GetOrBool("estimateContentLength", false)

	client, _ := params.Get("c")

	if format == "raw" || urlPath == "/download" {
		log.Printf("serving raw file %q for user %q client %q", audioFile.AudioFilename(), user.Name, client)
		http.ServeFile(w, r, file.AbsPath()) //nolint:gosec // path is from db, populated by scanner
		return nil
	}

	if maxBitRate >= audioFile.AudioBitrate() {
		log.Printf("serving raw file %q for user %q client %q, requested max bitrate %d is greater or equal to %d", audioFile.AudioFilename(), user.Name, client, maxBitRate, audioFile.AudioBitrate())
		http.ServeFile(w, r, file.AbsPath()) //nolint:gosec // path is from db, populated by scanner
		return nil
	}

	profile, clientChose, ok, err := streamBaseProfile(c.dbc, user.ID, client, format, maxBitRate)
	if err != nil {
		return spec.NewError(0, "couldn't pick profile: %v", err)
	}
	if !ok {
		log.Printf("serving raw file %q for user %q client %q, no matching profile for format %q", audioFile.AudioFilename(), user.Name, client, format)
		http.ServeFile(w, r, file.AbsPath()) //nolint:gosec // path is from db, populated by scanner
		return nil
	}

	// maxBitRate is the target when the client picked the profile, otherwise a cap
	if maxBitRate > 0 && (clientChose || int(profile.BitRate()) > maxBitRate) {
		profile = transcode.WithBitrate(profile, transcode.BitRate(maxBitRate))
	}
	if timeOffset > 0 {
		profile = transcode.WithSeek(profile, time.Second*time.Duration(timeOffset))
	}

	w.Header().Set("Content-Type", profile.MIME())

	if ct, ok := c.transcoder.(*transcode.CachingTranscoder); ok {
		path, release, err := ct.CachedPath(profile, file.AbsPath())
		if err != nil {
			return spec.NewError(0, "check transcode cache: %v", err)
		}
		if path != "" {
			defer release()
			log.Printf("serving cached transcode of %q to %q at bitrate %d for user %q client %q", audioFile.AudioFilename(), profile.MIME(), profile.BitRate(), user.Name, client)
			http.ServeFile(w, r, path) //nolint:gosec // path is a cache filename derived from an md5 key
			return nil
		}
	}

	log.Printf("transcoding %q to %q at bitrate %d for user %q client %q", audioFile.AudioFilename(), profile.MIME(), profile.BitRate(), user.Name, client)

	var out io.Writer = w
	if estimateLength {
		if rem := audioFile.AudioLength() - timeOffset; rem > 0 {
			size := transcode.EstimateSize(profile, time.Second*time.Duration(rem))
			w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
			out = &sizedWriter{w: w, size: size}
		}
	}

	if err := c.transcoder.Transcode(r.Context(), profile, file.AbsPath(), out); err != nil && !errors.Is(err, transcode.ErrFFmpegKilled) && !errors.Is(err, context.Canceled) {
		return spec.NewError(0, "error transcoding: %v", err)
	}

	if sw, ok := out.(*sizedWriter); ok {
		if err := sw.pad(); err != nil {
			return spec.NewError(0, "error padding stream: %v", err)
		}
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func (c *Controller) ServeGetAvatar(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	username, err := params.Get("username")
	if err != nil {
		return spec.NewError(10, "please provide an `username` parameter")
	}
	reqUser, err := c.dbc.GetUserByName(username)
	if err != nil {
		return spec.NewError(0, "error finding user: %v", err)
	}
	if reqUser == nil {
		return spec.NewError(70, "couldn't find a user with that name")
	}
	if (user.ID != reqUser.ID) && !user.IsAdmin {
		return spec.NewError(50, "user not admin")
	}
	http.ServeContent(w, r, "", time.Now(), bytes.NewReader(reqUser.Avatar))
	return nil
}

//nolint:gochecknoglobals
var defaultFormat = transcode.Opus

func streamBaseProfile(dbc *db.DB, userID int, client string, format string, maxBitRate int) (transcode.Profile, bool, bool, error) {
	prefName, prefProfile, err := streamGetTranscodePreference(dbc, userID, client)
	if err != nil {
		return transcode.Profile{}, false, false, err
	}

	// per-client override wins if it matches the client's format (or no format was requested)
	if prefName != "" && (format == "" || prefProfile.Suffix() == format) {
		return prefProfile, false, true, nil
	}

	// user configured format defaults. only consulted when the client requested a specific format
	if format != "" {
		var formatPrefs []*db.TranscodeFormatPreference
		if err := dbc.Where("user_id=?", userID).Order("created_at").Find(&formatPrefs).Error; err != nil {
			return transcode.Profile{}, false, false, fmt.Errorf("find format prefs: %w", err)
		}
		for _, fp := range formatPrefs {
			p, found := transcode.UserProfiles[fp.Profile]
			if !found {
				return transcode.Profile{}, false, false, fmt.Errorf("unknown transcode user profile %q", fp.Profile)
			}
			if p.Suffix() == format {
				return p, true, true, nil
			}
		}

		// hardcoded fallback for formats the user didn't configure
		defaults := []transcode.Profile{transcode.Opus, transcode.MP3}
		for _, p := range defaults {
			if p.Suffix() == format {
				return p, true, true, nil
			}
		}
	}

	if maxBitRate > 0 {
		return defaultFormat, true, true, nil
	}

	return transcode.Profile{}, false, false, nil
}

func streamGetTranscodePreference(dbc *db.DB, userID int, client string) (string, transcode.Profile, error) {
	var pref db.TranscodePreference
	err := dbc.
		Where("user_id=?", userID).
		Where("client COLLATE NOCASE IN (?)", []string{"*", client}).
		Order("client DESC"). // ensure "*" is last if it's there
		First(&pref).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", transcode.Profile{}, nil
	}
	if err != nil {
		return "", transcode.Profile{}, fmt.Errorf("find transcode preference: %w", err)
	}
	profile, ok := transcode.UserProfiles[pref.Profile]
	if !ok {
		return "", transcode.Profile{}, fmt.Errorf("unknown transcode user profile %q", pref.Profile)
	}
	return pref.Profile, profile, nil
}

func streamGetTranscodeMeta(dbc *db.DB, userID int, client string) spec.TranscodeMeta {
	prefName, profile, err := streamGetTranscodePreference(dbc, userID, client)
	if err != nil || prefName == "" {
		return spec.TranscodeMeta{}
	}
	return spec.TranscodeMeta{
		TranscodedContentType: profile.MIME(),
		TranscodedSuffix:      profile.Suffix(),
	}
}

type sizedWriter struct {
	w       io.Writer
	size    int64
	written int64
}

func (sw *sizedWriter) Write(p []byte) (int, error) {
	keep := p
	if rem := sw.size - sw.written; int64(len(keep)) > rem {
		keep = keep[:max(rem, 0)]
	}
	n, err := sw.w.Write(keep)
	sw.written += int64(n)
	if err != nil {
		return n, err
	}
	return len(p), nil
}

func (sw *sizedWriter) pad() error {
	if sw.written >= sw.size {
		return nil
	}
	_, err := io.CopyN(sw.w, zeroReader{}, sw.size-sw.written)
	return err
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
