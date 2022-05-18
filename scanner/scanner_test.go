package scanner_test

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/matryer/is"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/mockfs"
	"go.senan.xyz/gonic/multierr"
	"go.senan.xyz/gonic/scanner"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func TestTableCounts(t *testing.T) {
	t.Parallel()
	is := is.NewRelaxed(t)
	m := mockfs.New(t)

	m.AddItems()
	m.ScanAndClean()

	var tracks int
	is.NoErr(m.DB().Model(&db.Track{}).Count(&tracks).Error) // not all tracks
	is.Equal(tracks, m.NumTracks())

	var albums int
	is.NoErr(m.DB().Model(&db.Album{}).Count(&albums).Error) // not all albums
	is.Equal(albums, 13)                                     // not all albums

	var artists int
	is.NoErr(m.DB().Model(&db.Artist{}).Count(&artists).Error) // not all artists
	is.Equal(artists, 3)                                       // not all artists
}

func TestParentID(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	m := mockfs.New(t)

	m.AddItems()
	m.ScanAndClean()

	var nullParentAlbums []*db.Album
	is.NoErr(m.DB().Where("parent_id IS NULL").Find(&nullParentAlbums).Error) // one parent_id=NULL which is root folder
	is.Equal(len(nullParentAlbums), 1)                                        // one parent_id=NULL which is root folder
	is.Equal(nullParentAlbums[0].LeftPath, "")
	is.Equal(nullParentAlbums[0].RightPath, ".")

	is.Equal(m.DB().Where("id=parent_id").Find(&db.Album{}).Error, gorm.ErrRecordNotFound) // no self-referencing albums

	var album db.Album
	var parent db.Album
	is.NoErr(m.DB().Find(&album, "left_path=? AND right_path=?", "artist-0/", "album-0").Error) // album has parent ID
	is.NoErr(m.DB().Find(&parent, "right_path=?", "artist-0").Error)                            // album has parent ID
	is.Equal(album.ParentID, parent.ID)                                                         // album has parent ID
}

func TestUpdatedCover(t *testing.T) {
	t.Parallel()
	is := is.NewRelaxed(t)
	m := mockfs.New(t)

	m.AddItems()
	m.ScanAndClean()
	m.AddCover("artist-0/album-0/cover.jpg")
	m.ScanAndClean()

	var album db.Album
	is.NoErr(m.DB().Where("left_path=? AND right_path=?", "artist-0/", "album-0").Find(&album).Error) // album has cover
	is.Equal(album.Cover, "cover.jpg")                                                                // album has cover
}

func TestCoverBeforeTracks(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	m := mockfs.New(t)

	m.AddCover("artist-2/album-2/cover.jpg")
	m.ScanAndClean()
	m.AddItems()
	m.ScanAndClean()

	var album db.Album
	is.NoErr(m.DB().Preload("TagArtist").Where("left_path=? AND right_path=?", "artist-2/", "album-2").Find(&album).Error) // album has cover
	is.Equal(album.Cover, "cover.jpg")                                                                                     // album has cover
	is.Equal(album.TagArtist.Name, "artist-2")                                                                             // album artist

	var tracks []*db.Track
	is.NoErr(m.DB().Where("album_id=?", album.ID).Find(&tracks).Error) // album has tracks
	is.Equal(len(tracks), 3)                                           // album has tracks
}

func TestUpdatedTags(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	m := mockfs.New(t)

	m.AddTrack("artist-10/album-10/track-10.flac")
	m.SetTags("artist-10/album-10/track-10.flac", func(tags *mockfs.Tags) error {
		tags.RawArtist = "artist"
		tags.RawAlbumArtist = "album-artist"
		tags.RawAlbum = "album"
		tags.RawTitle = "title"
		return nil
	})

	m.ScanAndClean()

	var track db.Track
	is.NoErr(m.DB().Preload("Album").Preload("Artist").Where("filename=?", "track-10.flac").Find(&track).Error) // track has tags
	is.Equal(track.TagTrackArtist, "artist")                                                                    // track has tags
	is.Equal(track.Artist.Name, "album-artist")                                                                 // track has tags
	is.Equal(track.Album.TagTitle, "album")                                                                     // track has tags
	is.Equal(track.TagTitle, "title")                                                                           // track has tags

	m.SetTags("artist-10/album-10/track-10.flac", func(tags *mockfs.Tags) error {
		tags.RawArtist = "artist-upd"
		tags.RawAlbumArtist = "album-artist-upd"
		tags.RawAlbum = "album-upd"
		tags.RawTitle = "title-upd"
		return nil
	})

	m.ScanAndClean()

	var updated db.Track
	is.NoErr(m.DB().Preload("Album").Preload("Artist").Where("filename=?", "track-10.flac").Find(&updated).Error) // updated has tags
	is.Equal(updated.ID, track.ID)                                                                                // updated has tags
	is.Equal(updated.TagTrackArtist, "artist-upd")                                                                // updated has tags
	is.Equal(updated.Artist.Name, "album-artist-upd")                                                             // updated has tags
	is.Equal(updated.Album.TagTitle, "album-upd")                                                                 // updated has tags
	is.Equal(updated.TagTitle, "title-upd")                                                                       // updated has tags
}

// https://github.com/sentriz/gonic/issues/225
func TestUpdatedAlbumGenre(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	m := mockfs.New(t)

	m.AddItems()
	m.SetTags("artist-0/album-0/track-0.flac", func(tags *mockfs.Tags) error {
		tags.RawGenre = "gen-a;gen-b"
		return nil
	})

	m.ScanAndClean()

	var album db.Album
	is.NoErr(m.DB().Preload("Genres").Where("left_path=? AND right_path=?", "artist-0/", "album-0").Find(&album).Error)
	is.Equal(album.GenreStrings(), []string{"gen-a", "gen-b"})

	m.SetTags("artist-0/album-0/track-0.flac", func(tags *mockfs.Tags) error {
		tags.RawGenre = "gen-a-upd;gen-b-upd"
		return nil
	})

	m.ScanAndClean()

	var updated db.Album
	is.NoErr(m.DB().Preload("Genres").Where("left_path=? AND right_path=?", "artist-0/", "album-0").Find(&updated).Error)
	is.Equal(updated.GenreStrings(), []string{"gen-a-upd", "gen-b-upd"})
}

func TestDeleteAlbum(t *testing.T) {
	t.Parallel()
	is := is.NewRelaxed(t)
	m := mockfs.New(t)

	m.AddItems()
	m.ScanAndClean()

	is.NoErr(m.DB().Where("left_path=? AND right_path=?", "artist-2/", "album-2").Find(&db.Album{}).Error) // album exists

	m.RemoveAll("artist-2/album-2")
	m.ScanAndClean()

	is.Equal(m.DB().Where("left_path=? AND right_path=?", "artist-2/", "album-2").Find(&db.Album{}).Error, gorm.ErrRecordNotFound) // album doesn't exist
}

func TestDeleteArtist(t *testing.T) {
	t.Parallel()
	is := is.NewRelaxed(t)
	m := mockfs.New(t)

	m.AddItems()
	m.ScanAndClean()

	is.NoErr(m.DB().Where("left_path=? AND right_path=?", "artist-2/", "album-2").Find(&db.Album{}).Error) // album exists

	m.RemoveAll("artist-2")
	m.ScanAndClean()

	is.Equal(m.DB().Where("left_path=? AND right_path=?", "artist-2/", "album-2").Find(&db.Album{}).Error, gorm.ErrRecordNotFound) // album doesn't exist
	is.Equal(m.DB().Where("name=?", "artist-2").Find(&db.Artist{}).Error, gorm.ErrRecordNotFound)                                  // artist doesn't exist
}

func TestGenres(t *testing.T) {
	t.Parallel()
	is := is.New(t)
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
		is.Helper()
		is.NoErr(albumGenre(artist, album, genreName))
	}
	isAlbumGenreMissing := func(artist, album, genreName string) {
		is.Helper()
		is.Equal(albumGenre(artist, album, genreName), gorm.ErrRecordNotFound)
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
		is.Helper()
		is.NoErr(trackGenre(artist, album, filename, genreName))
	}
	isTrackGenreMissing := func(artist, album, filename, genreName string) {
		is.Helper()
		is.Equal(trackGenre(artist, album, filename, genreName), gorm.ErrRecordNotFound)
	}

	genre := func(genre string) error {
		return m.DB().Where("name=?", genre).Find(&db.Genre{}).Error
	}
	isGenre := func(genreName string) {
		is.Helper()
		is.NoErr(genre(genreName))
	}
	isGenreMissing := func(genreName string) {
		is.Helper()
		is.Equal(genre(genreName), gorm.ErrRecordNotFound)
	}

	m.AddItems()
	m.SetTags("artist-0/album-0/track-0.flac", func(tags *mockfs.Tags) error { tags.RawGenre = "genre-a;genre-b"; return nil })
	m.SetTags("artist-0/album-0/track-1.flac", func(tags *mockfs.Tags) error { tags.RawGenre = "genre-c;genre-d"; return nil })
	m.SetTags("artist-1/album-2/track-0.flac", func(tags *mockfs.Tags) error { tags.RawGenre = "genre-e;genre-f"; return nil })
	m.SetTags("artist-1/album-2/track-1.flac", func(tags *mockfs.Tags) error { tags.RawGenre = "genre-g;genre-h"; return nil })
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

	m.SetTags("artist-0/album-0/track-0.flac", func(tags *mockfs.Tags) error { tags.RawGenre = "genre-aa;genre-bb"; return nil })
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
	is := is.New(t)
	m := mockfs.NewWithDirs(t, []string{"m-1", "m-2", "m-3"})

	m.AddItemsPrefix("m-1")
	m.AddItemsPrefix("m-2")
	m.AddItemsPrefix("m-3")
	m.ScanAndClean()

	var rootDirs []*db.Album
	is.NoErr(m.DB().Where("parent_id IS NULL").Find(&rootDirs).Error)
	is.Equal(len(rootDirs), 3)
	for i, r := range rootDirs {
		is.Equal(r.RootDir, filepath.Join(m.TmpDir(), fmt.Sprintf("m-%d", i+1)))
		is.Equal(r.ParentID, 0)
		is.Equal(r.LeftPath, "")
		is.Equal(r.RightPath, ".")
	}

	m.AddCover("m-3/artist-0/album-0/cover.jpg")
	m.ScanAndClean()
	m.LogItems()

	checkCover := func(root string, q string) {
		is.Helper()
		is.NoErr(m.DB().Where(q, filepath.Join(m.TmpDir(), root)).Find(&db.Album{}).Error)
	}

	checkCover("m-1", "root_dir=? AND cover IS NULL")     // mf 1 no cover
	checkCover("m-2", "root_dir=? AND cover IS NULL")     // mf 2 no cover
	checkCover("m-3", "root_dir=? AND cover='cover.jpg'") // mf 3 has cover
}

func TestNewAlbumForExistingArtist(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	m := mockfs.New(t)

	m.AddItems()
	m.ScanAndClean()

	m.LogAlbums()
	m.LogArtists()

	var artist db.Artist
	is.NoErr(m.DB().Where("name=?", "artist-2").Find(&artist).Error) // find orig artist
	is.True(artist.ID > 0)

	for tr := 0; tr < 3; tr++ {
		m.AddTrack(fmt.Sprintf("artist-2/new-album/track-%d.mp3", tr))
		m.SetTags(fmt.Sprintf("artist-2/new-album/track-%d.mp3", tr), func(tags *mockfs.Tags) error {
			tags.RawArtist = "artist-2"
			tags.RawAlbumArtist = "artist-2"
			tags.RawAlbum = "new-album"
			tags.RawTitle = fmt.Sprintf("title-%d", tr)
			return nil
		})
	}

	var updated db.Artist
	is.NoErr(m.DB().Where("name=?", "artist-2").Find(&updated).Error) // find updated artist
	is.Equal(artist.ID, updated.ID)                                   // find updated artist

	var all []*db.Artist
	is.NoErr(m.DB().Find(&all).Error) // still only 3?
	is.Equal(len(all), 3)             // still only 3?
}

func TestMultiFolderWithSharedArtist(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	m := mockfs.NewWithDirs(t, []string{"m-0", "m-1"})

	const artistName = "artist-a"

	m.AddTrack(fmt.Sprintf("m-0/%s/album-a/track-1.flac", artistName))
	m.SetTags(fmt.Sprintf("m-0/%s/album-a/track-1.flac", artistName), func(tags *mockfs.Tags) error {
		tags.RawArtist = artistName
		tags.RawAlbumArtist = artistName
		tags.RawAlbum = "album-a"
		tags.RawTitle = "track-1"
		return nil
	})
	m.ScanAndClean()

	m.AddTrack(fmt.Sprintf("m-1/%s/album-a/track-1.flac", artistName))
	m.SetTags(fmt.Sprintf("m-1/%s/album-a/track-1.flac", artistName), func(tags *mockfs.Tags) error {
		tags.RawArtist = artistName
		tags.RawAlbumArtist = artistName
		tags.RawAlbum = "album-a"
		tags.RawTitle = "track-1"
		return nil
	})
	m.ScanAndClean()

	sq := func(db *gorm.DB) *gorm.DB {
		return db.
			Select("*, count(sub.id) child_count, sum(sub.length) duration").
			Joins("LEFT JOIN tracks sub ON albums.id=sub.album_id").
			Group("albums.id")
	}

	var artist db.Artist
	is.NoErr(m.DB().Where("name=?", artistName).Preload("Albums", sq).First(&artist).Error)
	is.Equal(artist.Name, artistName)
	is.Equal(len(artist.Albums), 2)

	for _, album := range artist.Albums {
		is.True(album.TagYear > 0)
		is.Equal(album.TagArtistID, artist.ID)
		is.True(album.ChildCount > 0)
		is.True(album.Duration > 0)
	}
}

func TestSymlinkedAlbum(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	m := mockfs.NewWithDirs(t, []string{"scan"})

	m.AddItemsPrefixWithCovers("temp")

	tempAlbum0 := filepath.Join(m.TmpDir(), "temp", "artist-0", "album-0")
	scanAlbum0 := filepath.Join(m.TmpDir(), "scan", "artist-sym", "album-0")
	m.Symlink(tempAlbum0, scanAlbum0)

	m.ScanAndClean()
	m.LogTracks()
	m.LogAlbums()

	var track db.Track
	is.NoErr(m.DB().Preload("Album.Parent").Find(&track).Error) // track exists
	is.True(track.Album != nil)                                 // track has album
	is.True(track.Album.Cover != "")                            // album has cover
	is.Equal(track.Album.Parent.RightPath, "artist-sym")        // artist is sym

	info, err := os.Stat(track.AbsPath())
	is.NoErr(err)                     // track resolves
	is.True(!info.IsDir())            // track resolves
	is.True(!info.ModTime().IsZero()) // track resolves
}

func TestSymlinkedSubdiscs(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	m := mockfs.NewWithDirs(t, []string{"scan"})

	addItem := func(prefix, artist, album, disc, track string) {
		p := fmt.Sprintf("%s/%s/%s/%s/%s", prefix, artist, album, disc, track)
		m.AddTrack(p)
		m.SetTags(p, func(tags *mockfs.Tags) error {
			tags.RawArtist = artist
			tags.RawAlbumArtist = artist
			tags.RawAlbum = album
			tags.RawTitle = track
			return nil
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
	is.NoErr(m.DB().Preload("Album.Parent").Find(&track).Error) // track exists
	is.True(track.Album != nil)                                 // track has album
	is.Equal(track.Album.Parent.RightPath, "album-sym")         // artist is sym

	info, err := os.Stat(track.AbsPath())
	is.NoErr(err)                     // track resolves
	is.True(!info.IsDir())            // track resolves
	is.True(!info.ModTime().IsZero()) // track resolves
}

func TestArtistHasCover(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	m := mockfs.New(t)

	m.AddItemsWithCovers()
	m.AddCover("artist-2/artist.png")
	m.ScanAndClean()

	var artistWith db.Artist
	is.NoErr(m.DB().Where("name=?", "artist-2").First(&artistWith).Error)
	is.Equal(artistWith.Cover, "artist.png")

	var artistWithout db.Artist
	is.NoErr(m.DB().Where("name=?", "artist-0").First(&artistWithout).Error)
	is.Equal(artistWithout.Cover, "")
}

func TestTagErrors(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	m := mockfs.New(t)

	m.AddItemsWithCovers()
	m.SetTags("artist-1/album-0/track-0.flac", func(tags *mockfs.Tags) error {
		return scanner.ErrReadingTags
	})
	m.SetTags("artist-1/album-1/track-0.flac", func(tags *mockfs.Tags) error {
		return scanner.ErrReadingTags
	})

	var errs *multierr.Err
	ctx, err := m.ScanAndCleanErr()
	is.True(errors.As(err, &errs))
	is.Equal(errs.Len(), 2)                            // we have 2 dir errors
	is.Equal(ctx.SeenTracks(), m.NumTracks()-(3*2))    // we saw all tracks bar 2 album contents
	is.Equal(ctx.SeenTracksNew(), m.NumTracks()-(3*2)) // we have all tracks bar 2 album contents

	ctx, err = m.ScanAndCleanErr()
	is.True(errors.As(err, &errs))
	is.Equal(errs.Len(), 2)                         // we have 2 dir errors
	is.Equal(ctx.SeenTracks(), m.NumTracks()-(3*2)) // we saw all tracks bar 2 album contents
	is.Equal(ctx.SeenTracksNew(), 0)                // we have no new tracks
}

// https://github.com/sentriz/gonic/issues/185#issuecomment-1050092128
func TestCompilationAlbumWithoutAlbumArtist(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	m := mockfs.New(t)

	const pathArtist = "various-artists"
	const pathAlbum = "my-compilation"
	const toAdd = 5

	// add tracks to one folder with random artists and no album artist tag
	for i := 0; i < toAdd; i++ {
		p := fmt.Sprintf("%s/%s/track-%d.flac", pathArtist, pathAlbum, i)
		m.AddTrack(p)
		m.SetTags(p, func(tags *mockfs.Tags) error {
			// don't set an album artist
			tags.RawTitle = fmt.Sprintf("track %d", i)
			tags.RawArtist = fmt.Sprintf("artist %d", i)
			tags.RawAlbum = pathArtist
			return nil
		})
	}

	m.ScanAndClean()

	var trackCount int
	is.NoErr(m.DB().Model(&db.Track{}).Count(&trackCount).Error)
	is.Equal(trackCount, 5)

	var artists []*db.Artist
	is.NoErr(m.DB().Preload("Albums").Find(&artists).Error)
	is.Equal(len(artists), 1)             // we only have one album artist
	is.Equal(artists[0].Name, "artist 0") // it came from the first track's fallback to artist tag
	is.Equal(len(artists[0].Albums), 1)   // the artist has one album
	is.Equal(artists[0].Albums[0].RightPath, pathAlbum)
	is.Equal(artists[0].Albums[0].LeftPath, pathArtist+"/")
}

func TestIncrementalScanNoChangeNoUpdatedAt(t *testing.T) {
	t.Parallel()
	is := is.New(t)
	m := mockfs.New(t)

	m.AddItems()

	m.ScanAndClean()
	var albumA db.Album
	is.NoErr(m.DB().Where("tag_artist_id NOT NULL").Order("updated_at DESC").Find(&albumA).Error)

	m.ScanAndClean()
	var albumB db.Album
	is.NoErr(m.DB().Where("tag_artist_id NOT NULL").Order("updated_at DESC").Find(&albumB).Error)

	is.Equal(albumA.UpdatedAt, albumB.UpdatedAt)
}
