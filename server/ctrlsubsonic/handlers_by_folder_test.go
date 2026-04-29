package ctrlsubsonic

import (
	"net/url"
	"path/filepath"
	"testing"

	"go.senan.xyz/gonic/db"
)

func TestGetIndexes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetIndexes, f.admin,
		query{url.Values{}, "no_args", false},
		query{url.Values{"musicFolderId": {"0"}}, "music_folder_0", false},
		query{url.Values{"musicFolderId": {"1"}}, "music_folder_1", false},
	)
}

func TestGetMusicDirectory(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetMusicDirectory, f.admin,
		query{url.Values{"id": {f.albumAA.SID().String()}}, "album_aa_leaf", false},
		// intermediate folder row -- no direct tracks, has child albums
		query{url.Values{"id": {f.folderID("m-0", "artist-a")}}, "artist_a_folder", false},
		query{url.Values{"id": {f.albumVA.SID().String()}}, "album_va_compilation", false},
		// Jamstash overrides flac -> mp3 since it can't play flac
		query{url.Values{"id": {f.albumAA.SID().String()}, "c": {"Jamstash"}}, "album_aa_jamstash", false},
	)
}

func TestGetArtistInfo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	// folder-mode getArtistInfo is a stub
	f.run(t, f.contr.ServeGetArtistInfo, f.admin,
		query{url.Values{}, "stub", false},
	)
}

func (f *fixture) folderID(rootDir, rightPath string) string {
	var a db.Album
	f.dbc.
		Where("right_path=? AND root_dir=?", rightPath, filepath.Join(f.m.TmpDir(), rootDir)).
		First(&a)
	return a.SID().String()
}

func TestGetAlbumList(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetAlbumList, f.admin,
		query{url.Values{"type": {"alphabeticalByArtist"}, "size": {"50"}}, "alpha_artist", false},
		query{url.Values{"type": {"alphabeticalByName"}, "size": {"50"}}, "alpha_name", false},
		query{url.Values{"type": {"byYear"}, "fromYear": {"2018"}, "toYear": {"2021"}, "size": {"50"}}, "by_year", false},
		query{url.Values{"type": {"byYear"}, "fromYear": {"2021"}, "toYear": {"2018"}, "size": {"50"}}, "by_year_inverted", false},
		query{url.Values{"type": {"byGenre"}, "genre": {"Rock"}, "size": {"50"}}, "by_genre_rock", false},
		query{url.Values{"type": {"byGenre"}, "genre": {"Jazz"}, "size": {"50"}}, "by_genre_jazz", false},
		query{url.Values{"type": {"frequent"}, "size": {"50"}}, "frequent", false},
		query{url.Values{"type": {"newest"}, "size": {"50"}}, "newest", false},
		query{url.Values{"type": {"random"}, "size": {"50"}}, "random", true},
		query{url.Values{"type": {"recent"}, "size": {"50"}}, "recent", false},
		query{url.Values{"type": {"starred"}, "size": {"50"}}, "starred_admin", false},
		query{url.Values{"type": {"highest"}, "size": {"50"}}, "highest_admin", false},
		query{url.Values{"type": {"alphabeticalByArtist"}, "musicFolderId": {"0"}, "size": {"50"}}, "alpha_artist_folder_0", false},
		query{url.Values{"type": {"alphabeticalByArtist"}, "musicFolderId": {"1"}, "size": {"50"}}, "alpha_artist_folder_1", false},
		query{url.Values{"type": {"garbage"}}, "unknown_type", false},
	)
	f.run(t, f.contr.ServeGetAlbumList, f.alt,
		query{url.Values{"type": {"starred"}, "size": {"50"}}, "starred_alt", false},
		query{url.Values{"type": {"highest"}, "size": {"50"}}, "highest_alt", false},
	)
}

func TestSearchTwo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeSearchTwo, f.admin,
		query{url.Values{"query": {"album"}}, "q_album", false},
		query{url.Values{"query": {"artist"}}, "q_artist", false},
		query{url.Values{"query": {"track"}}, "q_track", false},
		query{url.Values{"query": {"\"\""}}, "q_empty_all", false},
		query{url.Values{"query": {"00000000-0000-0000-0000-0000000000aa"}}, "q_uuid_album", false},
		query{url.Values{"query": {"album"}, "musicFolderId": {"1"}}, "q_album_folder_1", false},
	)
}

func TestGetStarred(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetStarred, f.admin,
		query{url.Values{}, "admin", false},
		query{url.Values{"musicFolderId": {"0"}}, "admin_folder_0", false},
		query{url.Values{"musicFolderId": {"1"}}, "admin_folder_1", false},
	)
	f.run(t, f.contr.ServeGetStarred, f.alt,
		query{url.Values{}, "alt", false},
	)
}
