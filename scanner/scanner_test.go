//nolint:goconst,errorlint
package scanner_test

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/mockfs"
	"go.senan.xyz/gonic/scanner"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func TestTableCounts(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	m.AddItems()
	m.ScanAndClean()

	var tracks int
	assert.NoError(t, m.DB().Model(&db.Track{}).Count(&tracks).Error) // not all tracks
	assert.Equal(t, tracks, m.NumTracks())

	var albums int
	assert.NoError(t, m.DB().Model(&db.Album{}).Count(&albums).Error) // not all albums
	assert.Equal(t, albums, 13)                                       // not all albums

	var artists int
	assert.NoError(t, m.DB().Model(&db.Artist{}).Count(&artists).Error) // not all artists
	assert.Equal(t, artists, 3)                                         // not all artists
}

func TestWithExcludePattern(t *testing.T) {
	t.Parallel()
	m := mockfs.NewWithExcludePattern(t, "\\/artist-1\\/|track-0.flac$")

	m.AddItems()
	m.ScanAndClean()

	var tracks int
	assert.NoError(t, m.DB().Model(&db.Track{}).Count(&tracks).Error) // not all tracks
	assert.Equal(t, tracks, 12)

	var albums int
	assert.NoError(t, m.DB().Model(&db.Album{}).Count(&albums).Error) // not all albums
	assert.Equal(t, albums, 10)                                       // not all albums

	var artists int
	assert.NoError(t, m.DB().Model(&db.Artist{}).Count(&artists).Error) // not all artists
	assert.Equal(t, artists, 2)                                         // not all artists
}

func TestParentID(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	m.AddItems()
	m.ScanAndClean()

	var nullParentAlbums []*db.Album
	assert.NoError(t, m.DB().Where("parent_id IS NULL").Find(&nullParentAlbums).Error) // one parent_id=NULL which is root folder
	assert.Equal(t, 1, len(nullParentAlbums))                                          // one parent_id=NULL which is root folder
	assert.Equal(t, "", nullParentAlbums[0].LeftPath)
	assert.Equal(t, ".", nullParentAlbums[0].RightPath)

	assert.Equal(t, gorm.ErrRecordNotFound, m.DB().Where("id=parent_id").Find(&db.Album{}).Error) // no self-referencing albums

	var album db.Album
	var parent db.Album
	assert.NoError(t, m.DB().Find(&album, "left_path=? AND right_path=?", "artist-0/", "album-0").Error) // album has parent ID
	assert.NoError(t, m.DB().Find(&parent, "right_path=?", "artist-0").Error)                            // album has parent ID
	assert.Equal(t, parent.ID, album.ParentID)                                                           // album has parent ID
}

func TestUpdatedCover(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	m.AddItems()
	m.ScanAndClean()
	m.AddCover("artist-0/album-0/cover.jpg")
	m.ScanAndClean()

	var album db.Album
	assert.NoError(t, m.DB().Where("left_path=? AND right_path=?", "artist-0/", "album-0").Find(&album).Error) // album has cover
	assert.Equal(t, album.Cover, "cover.jpg")                                                                  // album has cover
}

func TestCoverBeforeTracks(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	m.AddCover("artist-2/album-2/cover.jpg")
	m.ScanAndClean()
	m.AddItems()
	m.ScanAndClean()

	var album db.Album
	assert.NoError(t, m.DB().Where("left_path=? AND right_path=?", "artist-2/", "album-2").Find(&album).Error) // album has cover
	assert.Equal(t, "cover.jpg", album.Cover)                                                                  // album has cover

	var albumArtist db.Artist
	assert.NoError(t, m.DB().Joins("JOIN album_artists ON album_artists.artist_id=artists.id").Where("album_artists.album_id=?", album.ID).Find(&albumArtist).Error) // album has cover
	assert.Equal(t, "artist-2", albumArtist.Name)                                                                                                                    // album artist

	var tracks []*db.Track
	assert.NoError(t, m.DB().Where("album_id=?", album.ID).Find(&tracks).Error) // album has tracks
	assert.Equal(t, 3, len(tracks))                                             // album has tracks
}

func TestUpdatedTags(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	m.AddTrack("artist-10/album-10/track-10.flac")
	m.SetTags("artist-10/album-10/track-10.flac", func(tags *mockfs.TagInfo) {
		tags.RawArtist = "artist"
		tags.RawAlbumArtist = "album-artist"
		tags.RawAlbum = "album"
		tags.RawTitle = "title"
	})

	m.ScanAndClean()

	var track db.Track
	assert.NoError(t, m.DB().Preload("Album").Where("filename=?", "track-10.flac").Find(&track).Error) // track has tags
	assert.Equal(t, "artist", track.TagTrackArtist)                                                    // track has tags
	assert.Equal(t, "album", track.Album.TagTitle)                                                     // track has tags
	assert.Equal(t, "title", track.TagTitle)                                                           // track has tags

	var trackArtistA db.Artist
	assert.NoError(t, m.DB().Joins("JOIN album_artists ON album_artists.artist_id=artists.id").Where("album_artists.album_id=?", track.AlbumID).Limit(1).Find(&trackArtistA).Error) // updated has tags
	assert.Equal(t, "album-artist", trackArtistA.Name)                                                                                                                              // track has tags

	m.SetTags("artist-10/album-10/track-10.flac", func(tags *mockfs.TagInfo) {
		tags.RawArtist = "artist-upd"
		tags.RawAlbumArtist = "album-artist-upd"
		tags.RawAlbum = "album-upd"
		tags.RawTitle = "title-upd"
	})

	m.ScanAndClean()

	var updated db.Track
	assert.NoError(t, m.DB().Preload("Album").Where("filename=?", "track-10.flac").Find(&updated).Error) // updated has tags
	assert.Equal(t, track.ID, updated.ID)                                                                // updated has tags
	assert.Equal(t, "artist-upd", updated.TagTrackArtist)                                                // updated has tags
	assert.Equal(t, "album-upd", updated.Album.TagTitle)                                                 // updated has tags
	assert.Equal(t, "title-upd", updated.TagTitle)                                                       // updated has tags

	var trackArtistB db.Artist
	assert.NoError(t, m.DB().Joins("JOIN album_artists ON album_artists.artist_id=artists.id").Where("album_artists.album_id=?", track.AlbumID).Limit(1).Find(&trackArtistB).Error) // updated has tags
	assert.Equal(t, "album-artist-upd", trackArtistB.Name)                                                                                                                          // updated has tags
}

// https://github.com/sentriz/gonic/issues/225
func TestUpdatedAlbumGenre(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	m.AddItems()
	m.SetTags("artist-0/album-0/track-0.flac", func(tags *mockfs.TagInfo) {
		tags.RawGenre = "gen-a;gen-b"
	})

	m.ScanAndClean()

	var album db.Album
	assert.NoError(t, m.DB().Preload("Genres").Where("left_path=? AND right_path=?", "artist-0/", "album-0").Find(&album).Error)
	assert.Equal(t, []string{"gen-a", "gen-b"}, genreStrings(album))

	m.SetTags("artist-0/album-0/track-0.flac", func(tags *mockfs.TagInfo) {
		tags.RawGenre = "gen-a-upd;gen-b-upd"
	})

	m.ScanAndClean()

	var updated db.Album
	assert.NoError(t, m.DB().Preload("Genres").Where("left_path=? AND right_path=?", "artist-0/", "album-0").Find(&updated).Error)
	assert.Equal(t, []string{"gen-a-upd", "gen-b-upd"}, genreStrings(updated))
}

func genreStrings(album db.Album) []string {
	var strs []string
	for _, genre := range album.Genres {
		strs = append(strs, genre.Name)
	}
	return strs
}

func TestDeleteAlbum(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	m.AddItems()
	m.ScanAndClean()

	assert.NoError(t, m.DB().Where("left_path=? AND right_path=?", "artist-2/", "album-2").Find(&db.Album{}).Error) // album exists

	m.RemoveAll("artist-2/album-2")
	m.ScanAndClean()

	assert.Equal(t, m.DB().Where("left_path=? AND right_path=?", "artist-2/", "album-2").Find(&db.Album{}).Error, gorm.ErrRecordNotFound) // album doesn't exist
}

func TestDeleteArtist(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	m.AddItems()
	m.ScanAndClean()

	assert.NoError(t, m.DB().Where("left_path=? AND right_path=?", "artist-2/", "album-2").Find(&db.Album{}).Error) // album exists

	m.RemoveAll("artist-2")
	m.ScanAndClean()

	assert.Equal(t, m.DB().Where("left_path=? AND right_path=?", "artist-2/", "album-2").Find(&db.Album{}).Error, gorm.ErrRecordNotFound) // album doesn't exist
	assert.Equal(t, m.DB().Where("name=?", "artist-2").Find(&db.Artist{}).Error, gorm.ErrRecordNotFound)                                  // artist doesn't exist
}

func TestGenres(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	albumGenre := func(artist, album, genre string) error {
		return m.DB().
			Where("albums.left_path=? AND albums.right_path=? AND genres.name=?", artist, album, genre).
			Joins("JOIN albums ON albums.id=album_genres.album_id").
			Joins("JOIN genres ON genres.id=album_genres.genre_id").
			Find(&db.AlbumGenre{}).
			Error
	}
	isAlbumGenre := func(artist, album, genreName string) {
		assert.NoError(t, albumGenre(artist, album, genreName))
	}
	isAlbumGenreMissing := func(artist, album, genreName string) {
		assert.Equal(t, albumGenre(artist, album, genreName), gorm.ErrRecordNotFound)
	}

	trackGenre := func(artist, album, filename, genreName string) error {
		return m.DB().
			Where("albums.left_path=? AND albums.right_path=? AND tracks.filename=? AND genres.name=?", artist, album, filename, genreName).
			Joins("JOIN tracks ON tracks.id=track_genres.track_id").
			Joins("JOIN genres ON genres.id=track_genres.genre_id").
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Find(&db.TrackGenre{}).
			Error
	}
	isTrackGenre := func(artist, album, filename, genreName string) {
		assert.NoError(t, trackGenre(artist, album, filename, genreName))
	}
	isTrackGenreMissing := func(artist, album, filename, genreName string) {
		assert.Equal(t, trackGenre(artist, album, filename, genreName), gorm.ErrRecordNotFound)
	}

	genre := func(genre string) error {
		return m.DB().Where("name=?", genre).Find(&db.Genre{}).Error
	}
	isGenre := func(genreName string) {
		assert.NoError(t, genre(genreName))
	}
	isGenreMissing := func(genreName string) {
		assert.Equal(t, genre(genreName), gorm.ErrRecordNotFound)
	}

	m.AddItems()
	m.SetTags("artist-0/album-0/track-0.flac", func(tags *mockfs.TagInfo) { tags.RawGenre = "genre-a;genre-b" })
	m.SetTags("artist-0/album-0/track-1.flac", func(tags *mockfs.TagInfo) { tags.RawGenre = "genre-c;genre-d" })
	m.SetTags("artist-1/album-2/track-0.flac", func(tags *mockfs.TagInfo) { tags.RawGenre = "genre-e;genre-f" })
	m.SetTags("artist-1/album-2/track-1.flac", func(tags *mockfs.TagInfo) { tags.RawGenre = "genre-g;genre-h" })
	m.ScanAndClean()

	isGenre("genre-a") // genre exists
	isGenre("genre-b") // genre exists
	isGenre("genre-c") // genre exists
	isGenre("genre-d") // genre exists

	isTrackGenre("artist-0/", "album-0", "track-0.flac", "genre-a") // track genre exists
	isTrackGenre("artist-0/", "album-0", "track-0.flac", "genre-b") // track genre exists
	isTrackGenre("artist-0/", "album-0", "track-1.flac", "genre-c") // track genre exists
	isTrackGenre("artist-0/", "album-0", "track-1.flac", "genre-d") // track genre exists
	isTrackGenre("artist-1/", "album-2", "track-0.flac", "genre-e") // track genre exists
	isTrackGenre("artist-1/", "album-2", "track-0.flac", "genre-f") // track genre exists
	isTrackGenre("artist-1/", "album-2", "track-1.flac", "genre-g") // track genre exists
	isTrackGenre("artist-1/", "album-2", "track-1.flac", "genre-h") // track genre exists

	isAlbumGenre("artist-0/", "album-0", "genre-a") // album genre exists
	isAlbumGenre("artist-0/", "album-0", "genre-b") // album genre exists

	m.SetTags("artist-0/album-0/track-0.flac", func(tags *mockfs.TagInfo) { tags.RawGenre = "genre-aa;genre-bb" })
	m.ScanAndClean()

	isTrackGenre("artist-0/", "album-0", "track-0.flac", "genre-aa")       // updated track genre exists
	isTrackGenre("artist-0/", "album-0", "track-0.flac", "genre-bb")       // updated track genre exists
	isTrackGenreMissing("artist-0/", "album-0", "track-0.flac", "genre-a") // old track genre missing
	isTrackGenreMissing("artist-0/", "album-0", "track-0.flac", "genre-b") // old track genre missing

	isAlbumGenreMissing("artist-0/", "album-0", "genre-a") // old album genre missing
	isAlbumGenreMissing("artist-0/", "album-0", "genre-b") // old album genre missing

	isGenreMissing("genre-a") // old genre missing
	isGenreMissing("genre-b") // old genre missing
}

func TestMultiFolders(t *testing.T) {
	t.Parallel()
	m := mockfs.NewWithDirs(t, []string{"m-1", "m-2", "m-3"})

	m.AddItemsPrefix("m-1")
	m.AddItemsPrefix("m-2")
	m.AddItemsPrefix("m-3")
	m.ScanAndClean()

	var rootDirs []*db.Album
	assert.NoError(t, m.DB().Where("parent_id IS NULL").Find(&rootDirs).Error)
	assert.Equal(t, 3, len(rootDirs))
	for i, r := range rootDirs {
		assert.Equal(t, filepath.Join(m.TmpDir(), fmt.Sprintf("m-%d", i+1)), r.RootDir)
		assert.Equal(t, 0, r.ParentID)
		assert.Equal(t, "", r.LeftPath)
		assert.Equal(t, ".", r.RightPath)
	}

	m.AddCover("m-3/artist-0/album-0/cover.jpg")
	m.ScanAndClean()
	m.LogItems()

	checkCover := func(root string, q string) {
		assert.NoError(t, m.DB().Where(q, filepath.Join(m.TmpDir(), root)).Find(&db.Album{}).Error)
	}

	checkCover("m-1", "root_dir=? AND cover IS NULL")     // mf 1 no cover
	checkCover("m-2", "root_dir=? AND cover IS NULL")     // mf 2 no cover
	checkCover("m-3", "root_dir=? AND cover='cover.jpg'") // mf 3 has cover
}

func TestNewAlbumForExistingArtist(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	m.AddItems()
	m.ScanAndClean()

	m.LogAlbums()
	m.LogArtists()

	var artist db.Artist
	assert.NoError(t, m.DB().Where("name=?", "artist-2").Find(&artist).Error) // find orig artist
	assert.Greater(t, artist.ID, 0)

	for tr := 0; tr < 3; tr++ {
		m.AddTrack(fmt.Sprintf("artist-2/new-album/track-%d.mp3", tr))
		m.SetTags(fmt.Sprintf("artist-2/new-album/track-%d.mp3", tr), func(tags *mockfs.TagInfo) {
			tags.RawArtist = "artist-2"
			tags.RawAlbumArtist = "artist-2"
			tags.RawAlbum = "new-album"
			tags.RawTitle = fmt.Sprintf("title-%d", tr)
		})
	}

	var updated db.Artist
	assert.NoError(t, m.DB().Where("name=?", "artist-2").Find(&updated).Error) // find updated artist
	assert.Equal(t, updated.ID, artist.ID)                                     // find updated artist

	var all []*db.Artist
	assert.NoError(t, m.DB().Find(&all).Error) // still only 3?
	assert.Equal(t, 3, len(all))               // still only 3?
}

func TestMultiFolderWithSharedArtist(t *testing.T) {
	t.Parallel()
	m := mockfs.NewWithDirs(t, []string{"m-0", "m-1"})

	const artistName = "artist-a"

	m.AddTrack(fmt.Sprintf("m-0/%s/album-a/track-1.flac", artistName))
	m.SetTags(fmt.Sprintf("m-0/%s/album-a/track-1.flac", artistName), func(tags *mockfs.TagInfo) {
		tags.RawArtist = artistName
		tags.RawAlbumArtist = artistName
		tags.RawAlbum = "album-a"
		tags.RawTitle = "track-1"
	})
	m.ScanAndClean()

	m.AddTrack(fmt.Sprintf("m-1/%s/album-a/track-1.flac", artistName))
	m.SetTags(fmt.Sprintf("m-1/%s/album-a/track-1.flac", artistName), func(tags *mockfs.TagInfo) {
		tags.RawArtist = artistName
		tags.RawAlbumArtist = artistName
		tags.RawAlbum = "album-a"
		tags.RawTitle = "track-1"
	})
	m.ScanAndClean()

	var artist db.Artist
	assert.NoError(t, m.DB().Where("name=?", artistName).First(&artist).Error)
	assert.Equal(t, artistName, artist.Name)

	var artistAlbums []*db.Album
	assert.NoError(t, m.DB().
		Select("*, count(sub.id) child_count, sum(sub.length) duration").
		Joins("JOIN album_artists ON album_artists.album_id=albums.id").
		Joins("LEFT JOIN tracks sub ON albums.id=sub.album_id").
		Where("album_artists.artist_id=?", artist.ID).
		Group("albums.id").
		Find(&artistAlbums).Error)

	assert.Equal(t, 2, len(artistAlbums))

	for _, album := range artistAlbums {
		assert.Greater(t, album.TagYear, 0)
		assert.Greater(t, album.ChildCount, 0)
		assert.Greater(t, album.Duration, 0)
	}
}

func TestSymlinkedAlbum(t *testing.T) {
	t.Parallel()
	m := mockfs.NewWithDirs(t, []string{"scan"})

	m.AddItemsPrefixWithCovers("temp")

	tempAlbum0 := filepath.Join(m.TmpDir(), "temp", "artist-0", "album-0")
	scanAlbum0 := filepath.Join(m.TmpDir(), "scan", "artist-sym", "album-0")
	m.Symlink(tempAlbum0, scanAlbum0)

	m.ScanAndClean()
	m.LogTracks()
	m.LogAlbums()

	var track db.Track
	require.NoError(t, m.DB().Preload("Album.Parent").Find(&track).Error) // track exists
	require.NotNil(t, track.Album)                                        // track has album
	require.NotZero(t, track.Album.Cover)                                 // album has cover
	require.Equal(t, "artist-sym", track.Album.Parent.RightPath)          // artist is sym

	info, err := os.Stat(track.AbsPath())
	require.NoError(t, err)            // track resolves
	require.False(t, info.IsDir())     // track resolves
	require.NotZero(t, info.ModTime()) // track resolves
}

func TestSymlinkedSubdiscs(t *testing.T) {
	t.Parallel()
	m := mockfs.NewWithDirs(t, []string{"scan"})

	addItem := func(prefix, artist, album, disc, track string) {
		p := fmt.Sprintf("%s/%s/%s/%s/%s", prefix, artist, album, disc, track)
		m.AddTrack(p)
		m.SetTags(p, func(tags *mockfs.TagInfo) {
			tags.RawArtist = artist
			tags.RawAlbumArtist = artist
			tags.RawAlbum = album
			tags.RawTitle = track
		})
	}

	addItem("temp", "artist-a", "album-a", "disc-1", "track-1.flac")
	addItem("temp", "artist-a", "album-a", "disc-1", "track-2.flac")
	addItem("temp", "artist-a", "album-a", "disc-1", "track-3.flac")
	addItem("temp", "artist-a", "album-a", "disc-2", "track-1.flac")
	addItem("temp", "artist-a", "album-a", "disc-2", "track-2.flac")
	addItem("temp", "artist-a", "album-a", "disc-2", "track-3.flac")

	tempAlbum0 := filepath.Join(m.TmpDir(), "temp", "artist-a", "album-a")
	scanAlbum0 := filepath.Join(m.TmpDir(), "scan", "artist-a", "album-sym")
	m.Symlink(tempAlbum0, scanAlbum0)

	m.ScanAndClean()
	m.LogTracks()
	m.LogAlbums()

	var track db.Track
	assert.NoError(t, m.DB().Preload("Album.Parent").Find(&track).Error) // track exists
	assert.NotNil(t, track.Album)                                        // track has album
	assert.Equal(t, "album-sym", track.Album.Parent.RightPath)           // artist is sym

	info, err := os.Stat(track.AbsPath())
	assert.NoError(t, err)            // track resolves
	assert.False(t, info.IsDir())     // track resolves
	assert.NotZero(t, info.ModTime()) // track resolves
}

func TestSymlinkEscapesMusicDirs(t *testing.T) {
	t.Parallel()
	m := mockfs.NewWithDirs(t, []string{"scandir"})

	require.NoError(t, os.MkdirAll(filepath.Join(m.TmpDir(), "otherdir", "artist", "album-test"), os.ModePerm))
	require.NoError(t, os.Symlink(
		filepath.Join(m.TmpDir(), "otherdir", "artist"),
		filepath.Join(m.TmpDir(), "scandir", "artist"),
	))

	m.ScanAndClean()

	var albums []*db.Album
	require.NoError(t, m.DB().Find(&albums).Error)
	require.Len(t, albums, 3)
}

func TestTagErrors(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	m.AddItemsWithCovers()
	m.SetTags("artist-1/album-0/track-0.flac", func(tags *mockfs.TagInfo) {
		tags.Error = scanner.ErrReadingTags
	})
	m.SetTags("artist-1/album-1/track-0.flac", func(tags *mockfs.TagInfo) {
		tags.Error = scanner.ErrReadingTags
	})

	ctx, err := m.ScanAndCleanErr()
	errs, ok := err.(interface{ Unwrap() []error })
	assert.True(t, ok)

	assert.ErrorAs(t, err, &errs)
	assert.Equal(t, 2, len(errs.Unwrap()))                    // we have 2 dir errors
	assert.Equal(t, m.NumTracks()-(3*2), ctx.SeenTracks())    // we saw all tracks bar 2 album contents
	assert.Equal(t, m.NumTracks()-(3*2), ctx.SeenTracksNew()) // we have all tracks bar 2 album contents

	ctx, err = m.ScanAndCleanErr()
	errs, ok = err.(interface{ Unwrap() []error })
	assert.True(t, ok)

	assert.Equal(t, 2, len(errs.Unwrap()))                 // we have 2 dir errors
	assert.Equal(t, m.NumTracks()-(3*2), ctx.SeenTracks()) // we saw all tracks bar 2 album contents
	assert.Equal(t, 0, ctx.SeenTracksNew())                // we have no new tracks
}

// https://github.com/sentriz/gonic/issues/185#issuecomment-1050092128
func TestCompilationAlbumWithoutAlbumArtist(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	const pathArtist = "various-artists"
	const pathAlbum = "my-compilation"
	const toAdd = 5

	// add tracks to one folder with random artists and no album artist tag
	for i := 0; i < toAdd; i++ {
		p := fmt.Sprintf("%s/%s/track-%d.flac", pathArtist, pathAlbum, i)
		m.AddTrack(p)
		m.SetTags(p, func(tags *mockfs.TagInfo) {
			// don't set an album artist
			tags.RawTitle = fmt.Sprintf("track %d", i)
			tags.RawArtist = fmt.Sprintf("artist %d", i)
			tags.RawAlbum = pathArtist
		})
	}

	m.ScanAndClean()

	var trackCount int
	assert.NoError(t, m.DB().Model(&db.Track{}).Count(&trackCount).Error)
	assert.Equal(t, 5, trackCount)

	var artists []*db.Artist
	assert.NoError(t, m.DB().Joins("JOIN album_artists ON album_artists.artist_id=artists.id").Group("artists.id").Find(&artists).Error)
	assert.Equal(t, 1, len(artists))             // we only have one album artist
	assert.Equal(t, "artist 0", artists[0].Name) // it came from the first track's fallback to artist tag

	var artistAlbums []*db.Album
	assert.NoError(t, m.DB().Joins("JOIN album_artists ON album_artists.album_id=albums.id").Where("album_artists.artist_id=?", artists[0].ID).Find(&artistAlbums).Error)
	assert.Equal(t, 1, len(artistAlbums)) // the artist has one album
	assert.Equal(t, pathAlbum, artistAlbums[0].RightPath)
	assert.Equal(t, pathArtist+"/", artistAlbums[0].LeftPath)
}

func TestIncrementalScanNoChangeNoUpdatedAt(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	m.AddItems()

	m.ScanAndClean()
	var albumA db.Album
	assert.NoError(t, m.DB().Joins("JOIN album_artists ON album_artists.album_id=albums.id").Order("updated_at DESC").Find(&albumA).Error)

	m.ScanAndClean()
	var albumB db.Album
	assert.NoError(t, m.DB().Joins("JOIN album_artists ON album_artists.album_id=albums.id").Order("updated_at DESC").Find(&albumB).Error)

	assert.Equal(t, albumB.UpdatedAt, albumA.UpdatedAt)
}

// https://github.com/sentriz/gonic/issues/230
func TestAlbumAndArtistSameNameWeirdness(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	const name = "same"

	add := func(path string, a ...interface{}) {
		m.AddTrack(fmt.Sprintf(path, a...))
		m.SetTags(fmt.Sprintf(path, a...), func(tags *mockfs.TagInfo) {})
	}

	add("an-artist/%s/track-1.flac", name)
	add("an-artist/%s/track-2.flac", name)
	add("%s/an-album/track-1.flac", name)
	add("%s/an-album/track-2.flac", name)

	m.ScanAndClean()

	var albums []*db.Album
	assert.NoError(t, m.DB().Find(&albums).Error)
	assert.Equal(t, len(albums), 5) // root, 2 artists, 2 albums
}

func TestNoOrphanedGenres(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	m.AddItems()
	m.SetTags("artist-0/album-0/track-0.flac", func(tags *mockfs.TagInfo) { tags.RawGenre = "genre-a;genre-b" })
	m.SetTags("artist-0/album-0/track-1.flac", func(tags *mockfs.TagInfo) { tags.RawGenre = "genre-c;genre-d" })
	m.SetTags("artist-1/album-2/track-0.flac", func(tags *mockfs.TagInfo) { tags.RawGenre = "genre-e;genre-f" })
	m.SetTags("artist-1/album-2/track-1.flac", func(tags *mockfs.TagInfo) { tags.RawGenre = "genre-g;genre-h" })
	m.ScanAndClean()

	m.RemoveAll("artist-0")
	m.RemoveAll("artist-1")
	m.RemoveAll("artist-2")
	m.ScanAndClean()

	var genreCount int
	assert.NoError(t, m.DB().Model(&db.Genre{}).Count(&genreCount).Error)
	assert.Equal(t, 0, genreCount)
}

func TestMultiArtistSupport(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	m.AddItemsGlob("artist-0/album-[012]/track-0.*")
	m.SetTags("artist-0/album-0/track-0.flac", func(tags *mockfs.TagInfo) {
		tags.RawAlbum = "Mutator"
		tags.RawAlbumArtists = []string{"Alan Vega", "Liz Lamere"}
	})
	m.SetTags("artist-0/album-1/track-0.flac", func(tags *mockfs.TagInfo) {
		tags.RawAlbum = "Dead Man"
		tags.RawAlbumArtists = []string{"Alan Vega", "Mercury Rev"}
	})
	m.SetTags("artist-0/album-2/track-0.flac", func(tags *mockfs.TagInfo) {
		tags.RawAlbum = "Yerself Is Steam"
		tags.RawAlbumArtist = "Mercury Rev"
	})

	m.ScanAndClean()

	var artists []*db.Artist
	assert.NoError(t, m.DB().Joins("JOIN album_artists ON album_artists.artist_id=artists.id").Group("artists.id").Find(&artists).Error)
	assert.Len(t, artists, 3) // alan, liz, mercury

	var albumArtists []*db.AlbumArtist
	assert.NoError(t, m.DB().Find(&albumArtists).Error)
	assert.Len(t, albumArtists, 5)

	type row struct{ Artist, Albums string }
	state := func() []row {
		var table []row
		assert.NoError(t, m.DB().
			Select("artists.name artist, group_concat(albums.tag_title, ';') albums").
			Model(db.Artist{}).
			Joins("JOIN album_artists ON album_artists.artist_id=artists.id").
			Joins("JOIN albums ON albums.id=album_artists.album_id").
			Order("artists.name, albums.tag_title").
			Group("artists.id").
			Scan(&table).
			Error)

		return table
	}

	assert.Equal(t,
		[]row{
			{"Alan Vega", "Mutator;Dead Man"},
			{"Liz Lamere", "Mutator"},
			{"Mercury Rev", "Dead Man;Yerself Is Steam"},
		},
		state())

	m.RemoveAll("artist-0/album-2")
	m.SetTags("artist-0/album-1/track-0.flac", func(tags *mockfs.TagInfo) {
		tags.RawAlbum = "Dead Man"
		tags.RawAlbumArtists = []string{"Alan Vega"}
	})

	m.ScanAndClean()

	assert.NoError(t, m.DB().Joins("JOIN album_artists ON album_artists.artist_id=artists.id").Group("artists.id").Find(&artists).Error)
	assert.Len(t, artists, 2) // alan, liz

	assert.NoError(t, m.DB().Find(&albumArtists).Error)
	assert.Len(t, albumArtists, 3)

	assert.Equal(t,
		[]row{
			{"Alan Vega", "Mutator;Dead Man"},
			{"Liz Lamere", "Mutator"},
		},
		state())
}

func TestMultiArtistPreload(t *testing.T) {
	t.Parallel()
	m := mockfs.New(t)

	m.AddItemsGlob("artist-0/album-[012]/track-0.*")
	m.SetTags("artist-0/album-0/track-0.flac", func(tags *mockfs.TagInfo) {
		tags.RawAlbum = "Mutator"
		tags.RawAlbumArtists = []string{"Alan Vega", "Liz Lamere"}
	})
	m.SetTags("artist-0/album-1/track-0.flac", func(tags *mockfs.TagInfo) {
		tags.RawAlbum = "Dead Man"
		tags.RawAlbumArtists = []string{"Alan Vega", "Mercury Rev"}
	})
	m.SetTags("artist-0/album-2/track-0.flac", func(tags *mockfs.TagInfo) {
		tags.RawAlbum = "Yerself Is Steam"
		tags.RawAlbumArtist = "Mercury Rev"
	})

	m.ScanAndClean()

	var albums []*db.Album
	assert.NoError(t, m.DB().Preload("Artists").Find(&albums).Error)
	assert.GreaterOrEqual(t, len(albums), 3)

	for _, album := range albums {
		switch album.TagTitle {
		case "Mutator":
			assert.Len(t, album.Artists, 2)
		case "Dead Man":
			assert.Len(t, album.Artists, 2)
		case "Yerself Is Steam":
			assert.Len(t, album.Artists, 1)
		}
	}

	var artists []*db.Artist
	assert.NoError(t, m.DB().Preload("Albums").Joins("JOIN album_artists ON album_artists.artist_id=artists.id").Group("artists.id").Find(&artists).Error)
	assert.Equal(t, 3, len(artists))

	for _, artist := range artists {
		switch artist.Name {
		case "Alan Vega":
			assert.Len(t, artist.Albums, 2)
		case "Mercury Rev":
			assert.Len(t, artist.Albums, 2)
		case "Liz Lamere":
			assert.Len(t, artist.Albums, 1)
		}
	}
}
