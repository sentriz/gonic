package ctrlsubsonic

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/djherbis/times"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/cache"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/playlist"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/tags"
	gonictaglib "go.senan.xyz/gonic/tags/taglib"
	"go.senan.xyz/taglib"
)

func TestCoverForPlaylist(t *testing.T) {
	t.Parallel()

	t.Run("cover found jpg", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		playlistDir := filepath.Join(tmp, "1")
		require.NoError(t, os.MkdirAll(playlistDir, 0o755))
		require.NoError(t, touch(filepath.Join(playlistDir, "test-playlist.jpg")))

		playlistID := playlistIDEncode("1/test-playlist.m3u")
		file, err := coverForPlaylist(store, playlistID)
		require.NoError(t, err)
		require.NotNil(t, file)
		require.NoError(t, file.Close())
	})

	t.Run("cover found png", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		playlistDir := filepath.Join(tmp, "1")
		require.NoError(t, os.MkdirAll(playlistDir, 0o755))
		require.NoError(t, touch(filepath.Join(playlistDir, "test-playlist.png")))

		playlistID := playlistIDEncode("1/test-playlist.m3u")
		file, err := coverForPlaylist(store, playlistID)
		require.NoError(t, err)
		require.NotNil(t, file)
		require.NoError(t, file.Close())
	})

	t.Run("cover not found", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		playlistDir := filepath.Join(tmp, "1")
		require.NoError(t, os.MkdirAll(playlistDir, 0o755))

		playlistID := playlistIDEncode("1/test-playlist.m3u")
		file, err := coverForPlaylist(store, playlistID)
		require.ErrorIs(t, err, errCoverEmpty)
		require.Nil(t, file)
	})

	t.Run("nested playlist path", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		playlistDir := filepath.Join(tmp, "1", "subfolder")
		require.NoError(t, os.MkdirAll(playlistDir, 0o755))
		require.NoError(t, touch(filepath.Join(playlistDir, "my-nested-playlist.jpg")))

		playlistID := playlistIDEncode("1/subfolder/my-nested-playlist.m3u")
		file, err := coverForPlaylist(store, playlistID)
		require.NoError(t, err)
		require.NotNil(t, file)
		require.NoError(t, file.Close())
	})

	t.Run("playlist directory missing", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		playlistID := playlistIDEncode("1/test-playlist.m3u")
		file, err := coverForPlaylist(store, playlistID)
		require.Error(t, err)
		require.Nil(t, file)
	})

	t.Run("different playlists different covers", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		playlistDir := filepath.Join(tmp, "1")
		require.NoError(t, os.MkdirAll(playlistDir, 0o755))
		require.NoError(t, touch(filepath.Join(playlistDir, "playlist-a.jpg")))
		require.NoError(t, touch(filepath.Join(playlistDir, "playlist-b.png")))

		fileA, err := coverForPlaylist(store, playlistIDEncode("1/playlist-a.m3u"))
		require.NoError(t, err)
		require.Equal(t, "playlist-a.jpg", filepath.Base(fileA.Name()))
		require.NoError(t, fileA.Close())

		fileB, err := coverForPlaylist(store, playlistIDEncode("1/playlist-b.m3u"))
		require.NoError(t, err)
		require.Equal(t, "playlist-b.png", filepath.Base(fileB.Name()))
		require.NoError(t, fileB.Close())
	})
}

func touch(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return f.Close()
}

//nolint:paralleltest // Fixture creation changes GORM's global callbacks.
func TestGetCoverArtSourceChanges(t *testing.T) {
	for _, kind := range []string{"album", "podcast", "episode", "playlist", "long playlist"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			f.contr.coverCache = cache.New(t.TempDir(), 10)
			path := filepath.Join(f.m.TmpDir(), "m-0", "artist-a", "album-ab", "cover.png")
			id := *f.albumAB.SID()
			switch kind {
			case "podcast", "episode":
				podcast := db.Podcast{RootDir: t.TempDir(), Image: "cover.png"}
				require.NoError(t, f.dbc.Create(&podcast).Error)
				path = filepath.Join(podcast.RootDir, podcast.Image)
				id = *podcast.SID()
				if kind == "episode" {
					episode := db.PodcastEpisode{PodcastID: podcast.ID}
					require.NoError(t, f.dbc.Create(&episode).Error)
					id = *episode.SID()
				}
			case "playlist", "long playlist":
				rel := fmt.Sprintf("%d/test.m3u", f.admin.ID)
				if kind == "long playlist" {
					rel = fmt.Sprintf("%d/%s/%s/test.m3u", f.admin.ID, strings.Repeat("a", 100), strings.Repeat("b", 100))
				}
				require.NoError(t, f.contr.playlistStore.Write(rel, &playlist.Playlist{Name: "test"}))
				id = playlistIDEncode(rel)
				path = filepath.Join(f.contr.playlistStore.BasePath(), filepath.Dir(rel), "test.png")
			}
			red := color.RGBA{R: 255, A: 255}
			blue := color.RGBA{B: 255, A: 255}
			stamp := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			writeCoverPNG(t, path, red, stamp)
			for _, size := range []int{2, 1000} {
				assertCoverColor(t, requestCover(t, f, id, size), red)
			}

			// Cache touches implement LRU; they must not be mistaken for source changes.
			entries, err := os.ReadDir(f.contr.coverCache.Path())
			if kind == "long playlist" {
				require.Greater(t, len(id.String()), 255)
			}
			require.NoError(t, err)
			require.Len(t, entries, 2)
			for _, entry := range entries {
				require.Less(t, len(entry.Name()), 255)
				file, err := os.OpenFile(filepath.Join(f.contr.coverCache.Path(), entry.Name()), os.O_APPEND|os.O_WRONLY, 0)
				require.NoError(t, err)
				_, err = file.WriteString("cache-reuse-marker")
				require.NoError(t, err)
				require.NoError(t, file.Close())
			}
			for _, size := range []int{2, 1000} {
				data := requestCover(t, f, id, size)
				require.True(t, bytes.HasSuffix(data, []byte("cache-reuse-marker")))
				assertCoverColor(t, data, red)
			}

			writeCoverPNG(t, path, blue, stamp.Add(time.Hour))
			for _, size := range []int{2, 1000} {
				assertCoverColor(t, requestCover(t, f, id, size), blue)
			}
			// An obsolete cache entry must not hide removal of the actual source.
			require.NoError(t, os.Remove(path))
			w, r := makeHTTPMock(url.Values{"id": {id.String()}}, f.admin)
			require.NotNil(t, f.contr.ServeGetCoverArt(w, r))
		})
	}
}

//nolint:paralleltest // Fixture creation changes GORM's global callbacks.
func TestGetCoverArtPreservedModificationTime(t *testing.T) {
	f := newFixture(t)
	f.contr.coverCache = cache.New(t.TempDir(), 10)
	path := filepath.Join(f.m.TmpDir(), "m-0", "artist-a", "album-ab", "cover.png")
	stamp := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	writeCoverPNG(t, path, red, stamp)
	before, err := os.Stat(path)
	require.NoError(t, err)
	if !times.Get(before).HasChangeTime() {
		t.Skip("filesystem does not expose change time")
	}
	assertCoverColor(t, requestCover(t, f, *f.albumAB.SID(), 2), red)
	// Some filesystems have coarse timestamps. Wait for an observed change,
	// rather than assuming successive writes have different ctimes.
	deadline := time.Now().Add(3 * time.Second)
	for {
		writeCoverPNG(t, path, blue, stamp)
		after, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, before.Size(), after.Size())
		require.Equal(t, before.ModTime(), after.ModTime())
		if !times.Get(before).ChangeTime().Equal(times.Get(after).ChangeTime()) {
			break
		}
		require.True(t, time.Now().Before(deadline), "filesystem change time did not advance")
		time.Sleep(20 * time.Millisecond)
	}
	assertCoverColor(t, requestCover(t, f, *f.albumAB.SID(), 2), blue)
}

type coverTagReader struct {
	tags.Reader
	readCover func(string) ([]byte, error)
}

func (r coverTagReader) ReadCover(path string) ([]byte, error) { return r.readCover(path) }

//nolint:paralleltest // Fixture creation changes GORM's global callbacks.
func TestGetCoverArtTrackRevision(t *testing.T) {
	f := newFixture(t)
	f.contr.coverCache = cache.New(t.TempDir(), 10)
	f.m.ScanAndClean() // Restore real scan timestamps after the fixture's date reset.
	var track db.Track
	require.NoError(t, f.dbc.Preload("Album").First(&track, f.trackAB1.ID).Error)
	red := coverPNG(t, color.RGBA{R: 255, A: 255})
	blue := coverPNG(t, color.RGBA{B: 255, A: 255})
	cover := red
	calls := 0
	f.contr.tagReader = coverTagReader{Reader: f.m.TagReader(), readCover: func(path string) ([]byte, error) {
		require.Equal(t, track.AbsPath(), path)
		calls++
		return cover, nil
	}}
	assertCoverColor(t, requestCover(t, f, *track.SID(), 2), color.RGBA{R: 255, A: 255})
	f.m.ScanAndClean()
	assertCoverColor(t, requestCover(t, f, *track.SID(), 2), color.RGBA{R: 255, A: 255})
	require.Equal(t, 1, calls, "unchanged incremental scans must preserve the cached cover")
	for range 2 {
		assertCoverColor(t, requestCover(t, f, *track.SID(), 1000), color.RGBA{R: 255, A: 255})
	}
	require.Equal(t, 3, calls, "oversized requests still extract before the effective-size lookup")
	// A rescan can observe new embedded artwork even with preserved file metadata.
	cover = blue
	require.NoError(t, f.dbc.Model(&track).UpdateColumn("updated_at", track.UpdatedAt.Add(time.Hour)).Error)
	assertCoverColor(t, requestCover(t, f, *track.SID(), 2), color.RGBA{B: 255, A: 255})
	require.Equal(t, 4, calls)

	t.Run("replacement during extraction", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not support replacing this open file")
		}
		// The tag reader reopens the media path. Reject an atomic replacement
		// instead of saving new pixels under the previous source version.
		require.NoError(t, f.dbc.Model(&track).UpdateColumn("updated_at", track.UpdatedAt.Add(2*time.Hour)).Error)
		f.contr.tagReader = coverTagReader{Reader: f.m.TagReader(), readCover: func(path string) ([]byte, error) {
			replacement := path + ".replacement"
			require.NoError(t, os.WriteFile(replacement, []byte("replacement media"), 0o600))
			require.NoError(t, os.Rename(replacement, path))
			return red, nil
		}}
		w, r := makeHTTPMock(url.Values{"id": {track.SID().String()}, "size": {"2"}}, f.admin)
		require.NotNil(t, f.contr.ServeGetCoverArt(w, r))
		f.contr.tagReader = coverTagReader{Reader: f.m.TagReader(), readCover: func(string) ([]byte, error) { return blue, nil }}
		assertCoverColor(t, requestCover(t, f, *track.SID(), 2), color.RGBA{B: 255, A: 255})
	})
	require.NoError(t, os.Remove(track.AbsPath()))
	w, r := makeHTTPMock(url.Values{"id": {track.SID().String()}, "size": {"2"}}, f.admin)
	require.NotNil(t, f.contr.ServeGetCoverArt(w, r), "a cached track image must not hide a missing media file")
}

//nolint:paralleltest // Fixture creation changes GORM's global callbacks.
func TestCoverSourceRetainsSelectedPlaylistFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support replacing this open file")
	}
	f := newFixture(t)
	id := playlistIDEncode(fmt.Sprintf("%d/test.m3u", f.admin.ID))
	path := filepath.Join(f.contr.playlistStore.BasePath(), fmt.Sprint(f.admin.ID), "test.png")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	writeCoverPNG(t, path, red, time.Now())
	source, err := f.contr.coverSourceFor(id)
	require.NoError(t, err)
	defer source.file.Close()
	// Replace the path after selection. The retained descriptor still reads red,
	// and the change check prevents publishing it under a mismatched identity.
	replacement := path + ".replacement"
	writeCoverPNG(t, replacement, blue, time.Now())
	require.NoError(t, os.Rename(replacement, path))
	reader, err := source.reader(f.contr, id)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	assertCoverColor(t, data, red)
	require.Error(t, source.checkUnchanged())
}

//nolint:paralleltest // Fixture creation changes GORM's global callbacks.
func TestGetCoverArtCachedArtist(t *testing.T) {
	f := newFixture(t)
	f.contr.coverCache = cache.New(t.TempDir(), 10)
	id := *f.artistA.SID()
	col := color.RGBA{G: 255, A: 255}
	path := filepath.Join(f.contr.coverCache.Path(), coverCacheFilename(id.String(), 2, "png"))
	writeCoverPNG(t, path, col, time.Now())
	// An artist hit must not require a local source or fetch the remote artwork.
	assertCoverColor(t, requestCover(t, f, id, 2), col)
}

//nolint:paralleltest // Fixture creation changes GORM's global callbacks.
func TestGetCoverArtEmbeddedImage(t *testing.T) {
	f := newFixture(t)
	f.contr.coverCache = cache.New(t.TempDir(), 10)
	f.contr.tagReader = gonictaglib.Reader{}
	var track db.Track
	require.NoError(t, f.dbc.Preload("Album").First(&track, f.trackAB1.ID).Error)
	require.NoError(t, f.dbc.Model(&track).UpdateColumn("filename", "embedded.wav").Error)
	// Generate a minimal PCM WAV in the test; no external recording is needed.
	var wav bytes.Buffer
	wav.WriteString("RIFF")
	require.NoError(t, binary.Write(&wav, binary.LittleEndian, uint32(44)))
	wav.WriteString("WAVEfmt ")
	require.NoError(t, binary.Write(&wav, binary.LittleEndian, uint32(16)))
	for _, field := range []uint16{1, 1} {
		require.NoError(t, binary.Write(&wav, binary.LittleEndian, field))
	}
	for _, field := range []uint32{8000, 16000} {
		require.NoError(t, binary.Write(&wav, binary.LittleEndian, field))
	}
	for _, field := range []uint16{2, 16} {
		require.NoError(t, binary.Write(&wav, binary.LittleEndian, field))
	}
	wav.WriteString("data")
	require.NoError(t, binary.Write(&wav, binary.LittleEndian, uint32(8)))
	wav.Write(make([]byte, 8))
	require.NoError(t, os.WriteFile(track.AbsPath(), wav.Bytes(), 0o600))
	for i, col := range []color.RGBA{{R: 255, A: 255}, {B: 255, A: 255}} {
		require.NoError(t, taglib.WriteImage(track.AbsPath(), coverPNG(t, col)))
		stamp := time.Date(2020, 1, 1, i, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(track.AbsPath(), stamp, stamp))
		assertCoverColor(t, requestCover(t, f, *track.SID(), 2), col)
	}
}

func requestCover(t *testing.T, f *fixture, id specid.ID, size int) []byte {
	t.Helper()
	w, r := makeHTTPMock(url.Values{"id": {id.String()}, "size": {fmt.Sprint(size)}}, f.admin)
	r.URL.Path = "/rest/getCoverArt"
	require.Nil(t, f.contr.ServeGetCoverArt(w, r))
	require.Equal(t, 200, w.Code)
	return w.Body.Bytes()
}

func assertCoverColor(t *testing.T, data []byte, expected color.RGBA) {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, expected, color.RGBAModel.Convert(img.At(0, 0)))
}

func coverPNG(tb testing.TB, col color.RGBA) []byte {
	tb.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.SetRGBA(x, y, col)
		}
	}
	var out bytes.Buffer
	require.NoError(tb, png.Encode(&out, img))
	return out.Bytes()
}

func writeCoverPNG(t *testing.T, path string, col color.RGBA, stamp time.Time) {
	t.Helper()
	data := coverPNG(t, col)
	// Valid PNG decoders ignore trailing bytes; padding keeps sizes equal so the
	// preserved-mtime test isolates change time rather than file length.
	data = append(data, make([]byte, 4096-len(data))...)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	require.NoError(t, os.Chtimes(path, stamp, stamp))
}

func BenchmarkGetCoverArtCacheHit(b *testing.B) {
	// Share one database across calibration runs: db.New replaces a global GORM callback.
	f := newFixture(b)
	f.contr.coverCache = cache.New(b.TempDir(), 10)
	cover := coverPNG(b, color.RGBA{R: 255, A: 255})
	path := filepath.Join(f.m.TmpDir(), "m-0", "artist-a", "album-ab", "cover.png")
	require.NoError(b, os.WriteFile(path, cover, 0o600))
	f.contr.tagReader = coverTagReader{Reader: f.m.TagReader(), readCover: func(string) ([]byte, error) {
		return cover, nil
	}}
	for _, kind := range []string{"album", "track"} {
		b.Run(kind, func(b *testing.B) {
			id := f.albumAB.SID().String()
			if kind == "track" {
				id = f.trackAB1.SID().String()
			}
			warm, req := makeHTTPMock(url.Values{"id": {id}, "size": {"2"}}, f.admin)
			req.URL.Path = "/rest/getCoverArt"
			require.Nil(b, f.contr.ServeGetCoverArt(warm, req))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				w := httptest.NewRecorder()
				if err := f.contr.ServeGetCoverArt(w, req); err != nil {
					b.Fatal(err)
				}
				if w.Code != 200 || w.Body.Len() == 0 {
					b.Fatal("invalid cached response")
				}
			}
		})
	}
}
