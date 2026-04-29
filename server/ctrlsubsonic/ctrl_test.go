//nolint:thelper
package ctrlsubsonic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	jd "github.com/josephburnett/jd/lib"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/wrtag/tags/normtag"

	_ "go.senan.xyz/gonic/deps"

	"go.senan.xyz/gonic"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/infocache/albuminfocache"
	"go.senan.xyz/gonic/infocache/artistinfocache"
	"go.senan.xyz/gonic/mockfs"
	playlistp "go.senan.xyz/gonic/playlist"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/transcode"
)

func TestMain(m *testing.M) {
	gonic.Version = ""
	log.SetOutput(io.Discard)
	time.Local = time.UTC //nolint:gosmopolitan
	os.Exit(m.Run())
}

var testCamelExpr = regexp.MustCompile("([a-z0-9])([A-Z])")

const mockClientName = "test"

type query struct {
	params     url.Values
	expectPath string
	listSet    bool // compare lists as sets (use for random orderings)
}

func makeGoldenPath(test string) string {
	snake := testCamelExpr.ReplaceAllString(test, "${1}_${2}")
	lower := strings.ToLower(snake)
	relPath := strings.ReplaceAll(lower, "/", "_")
	return filepath.Join("testdata", relPath)
}

func makeHTTPMock(query url.Values, user *db.User) (*httptest.ResponseRecorder, *http.Request) {
	query.Add("f", "json")
	query.Add("u", user.Name)
	query.Add("p", user.Password)
	query.Add("v", "1")
	query.Add("c", mockClientName)
	req, _ := http.NewRequest("", "", nil)
	req.URL.RawQuery = query.Encode()
	ctx := req.Context()
	ctx = context.WithValue(ctx, CtxParams, params.New(req))
	ctx = context.WithValue(ctx, CtxUser, user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	return rr, req
}

// fixture is a controller + DB seeded with content shaped to hit every
// handler/scope edge case:
//
//	m-0/
//	  artist-a/
//	    album-aa/               # multi-genre, mb id, replay gain
//	      track-0.flac          # Rock;Pop
//	      track-1.flac          # Rock
//	      track-2.flac          # track-only, never reaches album. Jazz
//	    album-ab/               # multi-disc, folder cover, contributors
//	      cover.png
//	      d1-track-0.flac       # plural contributor tags
//	      d2-track-0.flac       # singular contributor tags
//	    empty-album/            # zero tracks, inserted post-scan
//	  artist-b/album-ba/        # singular credit-as
//	  collab-ab/album-collab/   # two album-artists, plural credit-as
//	  split-ab/album-split/     # two album-artists, no credit-as
//	  ärtist-c/album-ca/        # NameUDec
//	m-1/
//	  artist-a/album-cross/     # artist-a present in both folders
//	  various/comp/             # Various Artists, track artists differ
type fixture struct {
	contr *Controller
	m     *mockfs.MockFS
	dbc   *db.DB

	seq bool

	admin *db.User
	alt   *db.User

	artistA db.Artist
	artistB db.Artist
	artistC db.Artist // unicode name
	artistX db.Artist // only ever a track artist

	albumAA     db.Album
	albumAB     db.Album
	albumBA     db.Album
	albumCollab db.Album
	albumSplit  db.Album // multi album-artists, no credit-as
	albumCa     db.Album
	albumCross  db.Album
	albumVA     db.Album
	albumEmpty  db.Album

	trackAB1 db.Track
	trackVA0 db.Track
}

func newFixture(tb testing.TB) *fixture {
	tb.Helper()

	m := mockfs.NewWithDirs(tb, []string{"m-0", "m-1"})

	for tr := range 3 {
		path := fmt.Sprintf("m-0/artist-a/album-aa/track-%d.flac", tr)
		genre := "Rock"
		if tr == 0 {
			genre = "Rock;Pop"
		}
		if tr == 2 {
			genre = "Jazz" // track-only genre, won't reach album_genres (set from track 0)
		}
		m.SetTrack(path, func(info *mockfs.TagInfo) {
			normtag.Set(info.Tags, normtag.Artist, "artist-a")
			normtag.Set(info.Tags, normtag.AlbumArtist, "artist-a")
			normtag.Set(info.Tags, normtag.Album, "album-aa")
			normtag.Set(info.Tags, normtag.Title, fmt.Sprintf("title-%d", tr))
			normtag.Set(info.Tags, normtag.TrackNumber, fmt.Sprint(tr+1))
			normtag.Set(info.Tags, normtag.Date, "2018-06-01")
			normtag.Set(info.Tags, normtag.Genre, genre)
			normtag.Set(info.Tags, normtag.MusicBrainzReleaseID, "00000000-0000-0000-0000-0000000000aa")
			normtag.Set(info.Tags, normtag.MusicBrainzRecordingID, fmt.Sprintf("00000000-0000-0000-0000-aa00000000%02d", tr))
			normtag.Set(info.Tags, normtag.ReplayGainTrackGain, "-3.5 dB")
			normtag.Set(info.Tags, normtag.ReplayGainTrackPeak, "0.95")
			normtag.Set(info.Tags, normtag.ReplayGainAlbumGain, "-4.0 dB")
			normtag.Set(info.Tags, normtag.ReplayGainAlbumPeak, "0.99")
		})
	}

	m.AddCover("m-0/artist-a/album-ab/cover.png")
	// plural contributor tag forms, with and without credit-as
	m.SetTrack("m-0/artist-a/album-ab/d1-track-0.flac", func(info *mockfs.TagInfo) {
		normtag.Set(info.Tags, normtag.Artist, "artist-a")
		normtag.Set(info.Tags, normtag.AlbumArtist, "artist-a")
		normtag.Set(info.Tags, normtag.Album, "album-ab")
		normtag.Set(info.Tags, normtag.Title, "rich-title-d1")
		normtag.Set(info.Tags, normtag.TrackNumber, "1")
		normtag.Set(info.Tags, normtag.DiscNumber, "1")
		normtag.Set(info.Tags, "DISCSUBTITLE", "Disc One")
		normtag.Set(info.Tags, normtag.Date, "2019-04-01")
		normtag.Set(info.Tags, normtag.Genre, "Rock;Pop")
		normtag.Set(info.Tags, normtag.Composers, "comp-a", "comp-b")
		normtag.Set(info.Tags, normtag.ComposersCredit, "Composer A!", "Composer B!")
		normtag.Set(info.Tags, normtag.Remixers, "rem-a", "rem-b")
		normtag.Set(info.Tags, normtag.RemixersCredit, "Remixer A!", "Remixer B!")
		normtag.Set(info.Tags, normtag.Producers, "prod-a")
		normtag.Set(info.Tags, normtag.Lyricists, "lyr-a", "lyr-b")
		normtag.Set(info.Tags, normtag.Conductors, "cond-a")
		normtag.Set(info.Tags, normtag.Arrangers, "arr-a")
		normtag.Set(info.Tags, normtag.Lyrics, "[00:01.00]hello\n[00:02.00]world")
		normtag.Set(info.Tags, normtag.MusicBrainzReleaseID, "00000000-0000-0000-0000-0000000000ab")
		normtag.Set(info.Tags, normtag.MusicBrainzRecordingID, "00000000-0000-0000-0000-ab00000000d1")
		normtag.Set(info.Tags, normtag.ReleaseType, "EP")
	})
	// singular contributor tag forms, with and without credit-as
	m.SetTrack("m-0/artist-a/album-ab/d2-track-0.flac", func(info *mockfs.TagInfo) {
		normtag.Set(info.Tags, normtag.Artist, "artist-a")
		normtag.Set(info.Tags, normtag.AlbumArtist, "artist-a")
		normtag.Set(info.Tags, normtag.Album, "album-ab")
		normtag.Set(info.Tags, normtag.Title, "rich-title-d2")
		normtag.Set(info.Tags, normtag.TrackNumber, "1")
		normtag.Set(info.Tags, normtag.DiscNumber, "2")
		normtag.Set(info.Tags, "DISCSUBTITLE", "Disc Two")
		normtag.Set(info.Tags, normtag.Date, "2019-04-01")
		normtag.Set(info.Tags, normtag.Genre, "Pop")
		normtag.Set(info.Tags, normtag.Composer, "comp-c")
		normtag.Set(info.Tags, normtag.ComposerCredit, "Composer C!")
		normtag.Set(info.Tags, normtag.Remixer, "rem-c")
		normtag.Set(info.Tags, normtag.RemixerCredit, "Remixer C!")
		normtag.Set(info.Tags, normtag.Producer, "prod-c")
		normtag.Set(info.Tags, normtag.Lyricist, "lyr-c")
		normtag.Set(info.Tags, normtag.Conductor, "cond-c")
		normtag.Set(info.Tags, normtag.Arranger, "arr-c")
	})

	// singular ArtistCredit/AlbumArtistCredit (vs album-collab's plural form)
	m.SetTrack("m-0/artist-b/album-ba/track-0.flac", func(info *mockfs.TagInfo) {
		normtag.Set(info.Tags, normtag.Artist, "artist-b")
		normtag.Set(info.Tags, normtag.ArtistCredit, "The Mighty B")
		normtag.Set(info.Tags, normtag.AlbumArtist, "artist-b")
		normtag.Set(info.Tags, normtag.AlbumArtistCredit, "The Mighty B (LP)")
		normtag.Set(info.Tags, normtag.Album, "album-ba")
		normtag.Set(info.Tags, normtag.Title, "track-ba")
		normtag.Set(info.Tags, normtag.TrackNumber, "1")
		normtag.Set(info.Tags, normtag.Date, "2017-09-09")
		normtag.Set(info.Tags, normtag.Genre, "Jazz")
		normtag.Set(info.Tags, normtag.MusicBrainzReleaseID, "00000000-0000-0000-0000-0000000000ba")
		normtag.Set(info.Tags, normtag.ReleaseType, "Single")
	})

	// plural AlbumArtists/Artists with no credit-as (vs album-collab's credits)
	m.SetTrack("m-0/split-ab/album-split/track-0.flac", func(info *mockfs.TagInfo) {
		normtag.Set(info.Tags, normtag.Artists, "artist-a", "artist-b")
		normtag.Set(info.Tags, normtag.Artist, "artist-a")
		normtag.Set(info.Tags, normtag.AlbumArtists, "artist-a", "artist-b")
		normtag.Set(info.Tags, normtag.AlbumArtist, "artist-a")
		normtag.Set(info.Tags, normtag.Album, "album-split")
		normtag.Set(info.Tags, normtag.Title, "split-track")
		normtag.Set(info.Tags, normtag.TrackNumber, "1")
		normtag.Set(info.Tags, normtag.Date, "2023-03-03")
		normtag.Set(info.Tags, normtag.Genre, "Pop")
		normtag.Set(info.Tags, normtag.ReleaseType, "Album")
	})

	m.SetTrack("m-0/collab-ab/album-collab/track-0.flac", func(info *mockfs.TagInfo) {
		normtag.Set(info.Tags, normtag.Artist, "artist-a")
		normtag.Set(info.Tags, normtag.Artists, "artist-a", "artist-b")
		normtag.Set(info.Tags, normtag.ArtistsCredit, "Artist A!", "Artist B!")
		normtag.Set(info.Tags, normtag.AlbumArtist, "artist-a")
		normtag.Set(info.Tags, normtag.AlbumArtists, "artist-a", "artist-b")
		normtag.Set(info.Tags, normtag.AlbumArtistsCredit, "Artist A!", "Artist B!")
		normtag.Set(info.Tags, normtag.Album, "album-collab")
		normtag.Set(info.Tags, normtag.Title, "collab-track")
		normtag.Set(info.Tags, normtag.TrackNumber, "1")
		normtag.Set(info.Tags, normtag.Date, "2021-02-02")
		normtag.Set(info.Tags, normtag.Genre, "Pop")
		normtag.Set(info.Tags, normtag.ReleaseType, "Album")
	})

	m.SetTrack("m-0/ärtist-c/album-ca/track-0.flac", func(info *mockfs.TagInfo) {
		normtag.Set(info.Tags, normtag.Artist, "ärtist-c")
		normtag.Set(info.Tags, normtag.AlbumArtist, "ärtist-c")
		normtag.Set(info.Tags, normtag.Album, "album-ca")
		normtag.Set(info.Tags, normtag.Title, "track-ca")
		normtag.Set(info.Tags, normtag.TrackNumber, "1")
		normtag.Set(info.Tags, normtag.Date, "2016-01-01")
		normtag.Set(info.Tags, normtag.Genre, "Rock")
	})

	// plural Artists/AlbumArtists with a single value (single-element multi path)
	m.SetTrack("m-1/artist-a/album-cross/track-0.flac", func(info *mockfs.TagInfo) {
		normtag.Set(info.Tags, normtag.Artists, "artist-a")
		normtag.Set(info.Tags, normtag.AlbumArtists, "artist-a")
		normtag.Set(info.Tags, normtag.Artist, "artist-a") // fallback for MustArtist
		normtag.Set(info.Tags, normtag.AlbumArtist, "artist-a")
		normtag.Set(info.Tags, normtag.Album, "album-cross")
		normtag.Set(info.Tags, normtag.Title, "cross-track")
		normtag.Set(info.Tags, normtag.TrackNumber, "1")
		normtag.Set(info.Tags, normtag.Date, "2022-03-03")
		normtag.Set(info.Tags, normtag.Genre, "Rock")
	})

	m.SetTrack("m-1/various/comp/track-0.flac", func(info *mockfs.TagInfo) {
		normtag.Set(info.Tags, normtag.Artist, "artist-x")
		normtag.Set(info.Tags, normtag.AlbumArtist, "Various Artists")
		normtag.Set(info.Tags, normtag.Album, "comp")
		normtag.Set(info.Tags, normtag.Title, "comp-track-0")
		normtag.Set(info.Tags, normtag.TrackNumber, "1")
		normtag.Set(info.Tags, normtag.Date, "2020-01-01")
		normtag.Set(info.Tags, normtag.Genre, "Rock")
		normtag.Set(info.Tags, normtag.Compilation, "1")
		normtag.Set(info.Tags, normtag.ReleaseType, "Album, Compilation")
	})
	m.SetTrack("m-1/various/comp/track-1.flac", func(info *mockfs.TagInfo) {
		normtag.Set(info.Tags, normtag.Artist, "artist-y")
		normtag.Set(info.Tags, normtag.AlbumArtist, "Various Artists")
		normtag.Set(info.Tags, normtag.Album, "comp")
		normtag.Set(info.Tags, normtag.Title, "comp-track-1")
		normtag.Set(info.Tags, normtag.TrackNumber, "2")
		normtag.Set(info.Tags, normtag.Date, "2020-01-01")
		normtag.Set(info.Tags, normtag.Genre, "Jazz")
		normtag.Set(info.Tags, normtag.Compilation, "1")
		normtag.Set(info.Tags, normtag.ReleaseType, "Album, Compilation")
	})

	m.ScanAndClean()
	m.ResetDates()

	dbc := m.DB()

	admin := dbc.GetUserByName("admin")
	require.NotNil(tb, admin)

	alt := &db.User{Name: "alt", Password: "alt"}
	require.NoError(tb, dbc.Save(alt).Error)

	f := &fixture{m: m, dbc: dbc, admin: admin, alt: alt}

	dbc.Where("name=?", "artist-a").First(&f.artistA)
	dbc.Where("name=?", "artist-b").First(&f.artistB)
	dbc.Where("name=?", "ärtist-c").First(&f.artistC)
	dbc.Where("name=?", "artist-x").First(&f.artistX)

	dbc.Where("right_path=? AND tag_title=?", "album-aa", "album-aa").First(&f.albumAA)
	dbc.Where("right_path=? AND tag_title=?", "album-ab", "album-ab").First(&f.albumAB)
	dbc.Where("right_path=? AND tag_title=?", "album-ba", "album-ba").First(&f.albumBA)
	dbc.Where("right_path=? AND tag_title=?", "album-collab", "album-collab").First(&f.albumCollab)
	dbc.Where("right_path=? AND tag_title=?", "album-split", "album-split").First(&f.albumSplit)
	dbc.Where("right_path=? AND tag_title=?", "album-ca", "album-ca").First(&f.albumCa)
	dbc.Where("right_path=? AND tag_title=?", "album-cross", "album-cross").First(&f.albumCross)
	dbc.Where("right_path=? AND tag_title=?", "comp", "comp").First(&f.albumVA)

	dbc.
		Joins("JOIN albums ON albums.id=tracks.album_id").
		Where("tracks.filename=? AND albums.right_path=?", "d1-track-0.flac", "album-ab").
		First(&f.trackAB1)
	dbc.
		Joins("JOIN albums ON albums.id=tracks.album_id").
		Where("tracks.filename=? AND albums.right_path=?", "track-0.flac", "comp").
		First(&f.trackVA0)

	star1 := time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC)
	star2 := time.Date(2020, 6, 2, 12, 0, 0, 0, time.UTC)
	play1 := time.Date(2020, 7, 1, 12, 0, 0, 0, time.UTC)
	play2 := time.Date(2020, 8, 2, 12, 0, 0, 0, time.UTC)

	dbc.Save(&db.AlbumStar{UserID: admin.ID, AlbumID: f.albumAA.ID, StarDate: star1})
	dbc.Save(&db.AlbumStar{UserID: admin.ID, AlbumID: f.albumCollab.ID, StarDate: star2})
	dbc.Save(&db.AlbumRating{UserID: admin.ID, AlbumID: f.albumAA.ID, Rating: 4})
	f.albumAA.AverageRating = 4
	dbc.Save(&f.albumAA)

	dbc.Save(&db.ArtistStar{UserID: admin.ID, ArtistID: f.artistA.ID, StarDate: star1})
	dbc.Save(&db.ArtistRating{UserID: admin.ID, ArtistID: f.artistA.ID, Rating: 5})
	f.artistA.AverageRating = 5
	dbc.Save(&f.artistA)

	dbc.Save(&db.TrackStar{UserID: admin.ID, TrackID: f.trackAB1.ID, StarDate: star1})
	dbc.Save(&db.TrackRating{UserID: admin.ID, TrackID: f.trackAB1.ID, Rating: 3})
	f.trackAB1.AverageRating = 3
	dbc.Save(&f.trackAB1)

	dbc.Save(&db.Play{UserID: admin.ID, AlbumID: f.albumAA.ID, Time: play1, Count: 7, Length: 600})
	dbc.Save(&db.Play{UserID: admin.ID, AlbumID: f.albumAB.ID, Time: play2, Count: 2, Length: 200})

	dbc.Save(&db.AlbumStar{UserID: alt.ID, AlbumID: f.albumBA.ID, StarDate: star2})
	dbc.Save(&db.ArtistStar{UserID: alt.ID, ArtistID: f.artistC.ID, StarDate: star2})
	dbc.Save(&db.AlbumRating{UserID: alt.ID, AlbumID: f.albumAB.ID, Rating: 5})
	f.albumAB.AverageRating = 5
	dbc.Save(&f.albumAB)
	dbc.Save(&db.Play{UserID: alt.ID, AlbumID: f.albumCollab.ID, Time: play1, Count: 3, Length: 300})

	f.trackVA0.HasEmbeddedCover = true
	dbc.Save(&f.trackVA0)
	f.albumVA.EmbeddedCoverTrackID = &f.trackVA0.ID
	dbc.Save(&f.albumVA)

	var artistAFolder db.Album
	dbc.
		Where("right_path=? AND root_dir=?", "artist-a", filepath.Join(m.TmpDir(), "m-0")).
		First(&artistAFolder)
	f.albumEmpty = db.Album{
		LeftPath:       "artist-a/",
		RightPath:      "empty-album",
		RootDir:        filepath.Join(m.TmpDir(), "m-0"),
		ParentID:       artistAFolder.ID,
		TagTitle:       "empty-album",
		TagAlbumArtist: "artist-a",
		TagYear:        2015,
		TagReleaseType: "Album",
		CreatedAt:      time.Date(2020, 0, 0, 0, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2020, 0, 0, 0, 0, 0, 0, time.UTC),
		ModifiedAt:     time.Date(2020, 0, 0, 0, 0, 0, 0, time.UTC),
	}
	require.NoError(tb, dbc.Save(&f.albumEmpty).Error)
	require.NoError(tb, dbc.Save(&db.AlbumCredit{
		AlbumID: f.albumEmpty.ID, ArtistID: f.artistA.ID, Role: db.RoleAlbumArtist,
	}).Error)

	// pre-populated lastfm caches so getArtistInfo2/getAlbumInfo2 don't reach out
	require.NoError(tb, dbc.Save(&db.ArtistInfo{
		ID:             f.artistA.ID,
		Biography:      "an artist that exists.",
		MusicBrainzID:  "00000000-0000-0000-0000-aaaaaaaaaaaa",
		LastFMURL:      "https://example.invalid/artist-a",
		ImageURL:       "https://example.invalid/artist-a.jpg",
		SimilarArtists: "artist-b;artist-not-in-db",
		TopTracks:      "title-0;title-1",
		UpdatedAt:      time.Now(),
	}).Error)
	require.NoError(tb, dbc.Save(&db.AlbumInfo{
		ID:            f.albumAA.ID,
		Notes:         "an album that exists.",
		MusicBrainzID: "00000000-0000-0000-0000-aaaaaaaaaaab",
		LastFMURL:     "https://example.invalid/album-aa",
		UpdatedAt:     time.Now(),
	}).Error)

	musicPaths := []MusicPath{
		{Path: filepath.Join(m.TmpDir(), "m-0")},
		{Path: filepath.Join(m.TmpDir(), "m-1")},
	}

	playlistDir := filepath.Join(m.TmpDir(), "playlists")
	require.NoError(tb, os.MkdirAll(playlistDir, 0o755))
	playlistStore, err := playlistp.NewStore(playlistDir)
	require.NoError(tb, err)

	// shared playlist with a stable path so getPlaylists/getPlaylist read
	// deterministic content. mutation tests create their own.
	stableTime := time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(tb, playlistStore.Write(
		filepath.Join("1", "shared.m3u"),
		&playlistp.Playlist{
			UserID:    admin.ID,
			UpdatedAt: stableTime,
			Name:      "shared playlist",
			Comment:   "for testing",
			IsPublic:  true,
			Items: []string{
				filepath.Join(m.TmpDir(), "m-0", "artist-a", "album-aa", "track-0.flac"),
				filepath.Join(m.TmpDir(), "m-0", "artist-a", "album-aa", "track-1.flac"),
			},
		},
	))

	f.contr = &Controller{
		dbc:              dbc,
		musicPaths:       musicPaths,
		transcoder:       transcode.NewFFmpegTranscoder(),
		artistInfoCache:  artistinfocache.New(dbc, nil),
		albumInfoCache:   albuminfocache.New(dbc, nil),
		playlistStore:    playlistStore,
		resolveProxyPath: func(in string) string { return in },
	}
	return f
}

func (f *fixture) sharedPlaylistID() string {
	return playlistIDEncode(filepath.Join("1", "shared.m3u")).String()
}

func (f *fixture) query(t *testing.T, h handlerSubsonic, user *db.User, q url.Values) string {
	t.Helper()
	rr, req := makeHTTPMock(q, user)
	resp(h).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %q", rr.Code, rr.Body.String())
	}
	return rr.Body.String()
}

func (f *fixture) run(t *testing.T, h handlerSubsonic, user *db.User, cases ...query) {
	t.Helper()
	for _, qc := range cases {
		t.Run(qc.expectPath, func(t *testing.T) {
			t.Helper()
			if !f.seq {
				t.Parallel()
			}

			rr, req := makeHTTPMock(qc.params, user)
			resp(h).ServeHTTP(rr, req)
			body := rr.Body.String()
			if status := rr.Code; status != http.StatusOK {
				t.Fatalf("didn't give a 200\n%s", body)
			}

			goldenPath := makeGoldenPath(t.Name())
			goldenRegen := os.Getenv("GONIC_REGEN")
			if goldenRegen == "*" || (goldenRegen != "" && strings.HasPrefix(t.Name(), goldenRegen)) {
				var pretty bytes.Buffer
				out := []byte(body)
				if err := json.Indent(&pretty, out, "", "  "); err == nil {
					out = append(pretty.Bytes(), '\n')
				}
				_ = os.WriteFile(goldenPath, out, 0o600)
				t.Logf("golden file %q regenerated for %s", goldenPath, t.Name())
				t.SkipNow()
			}

			expected, err := jd.ReadJsonFile(goldenPath)
			if err != nil {
				t.Fatalf("parsing expected: %v", err)
			}
			actual, err := jd.ReadJsonString(body)
			if err != nil {
				t.Fatalf("parsing actual: %v", err)
			}
			diffOpts := []jd.Metadata{}
			if qc.listSet {
				diffOpts = append(diffOpts, jd.SET)
			}
			diff := expected.Diff(actual, diffOpts...)

			if len(diff) > 0 {
				t.Errorf("[31;1mhandler json differs from test json[0m")
				t.Errorf("[33;1mif you want to regenerate it, re-run with GONIC_REGEN=%s[0m\n", t.Name())
				t.Error(diff.Render())
			}
		})
	}
}

func TestParams(t *testing.T) {
	t.Parallel()

	handler := withParams(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := r.Context().Value(CtxParams).(params.Params)
		require.Equal(t, "Client", params.GetOr("c", ""))
	}))
	values := url.Values{}
	values.Set("c", "Client")

	r, err := http.NewRequest(http.MethodGet, "/?"+values.Encode(), nil)
	require.NoError(t, err)
	handler.ServeHTTP(nil, r)

	r, err = http.NewRequest(http.MethodPost, "/", strings.NewReader(values.Encode()))
	require.NoError(t, err)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(nil, r)
}
