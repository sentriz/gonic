package ctrlsubsonic

import (
	"net/url"
	"testing"
)

func TestGetArtists(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetArtists, f.admin,
		query{url.Values{}, "no_args", false},
		query{url.Values{"musicFolderId": {"0"}}, "music_folder_0", false},
		query{url.Values{"musicFolderId": {"1"}}, "music_folder_1", false},
	)
}

func TestGetArtist(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetArtist, f.admin,
		query{url.Values{"id": {f.artistA.SID().String()}}, "artist_a_multi_role", false},
		query{url.Values{"id": {f.artistB.SID().String()}}, "artist_b", false},
		// artist-x has track credits but no album credits -- exercises the
		// appearances UNION half
		query{url.Values{"id": {f.artistX.SID().String()}}, "artist_x_track_only", false},
		query{url.Values{"id": {f.artistC.SID().String()}}, "artist_c_unicode", false},
	)
}

func TestGetAlbum(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetAlbum, f.admin,
		query{url.Values{"id": {f.albumAA.SID().String()}}, "album_aa_starred_rated", false},
		query{url.Values{"id": {f.albumAB.SID().String()}}, "album_ab_multi_disc_contributors", false},
		query{url.Values{"id": {f.albumCollab.SID().String()}}, "album_collab_credit_as", false},
		query{url.Values{"id": {f.albumSplit.SID().String()}}, "album_split_multi_no_credit", false},
		query{url.Values{"id": {f.albumVA.SID().String()}}, "album_va_compilation", false},
		query{url.Values{"id": {f.albumEmpty.SID().String()}}, "album_empty_zero_tracks", false},
		// DSub suppresses the contributors list
		query{url.Values{"id": {f.albumAB.SID().String()}, "c": {"DSub"}}, "album_ab_dsub_no_contributors", false},
	)
}

func TestGetAlbumListTwo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetAlbumListTwo, f.admin,
		query{url.Values{"type": {"alphabeticalByArtist"}, "size": {"50"}}, "alpha_artist", false},
		query{url.Values{"type": {"alphabeticalByName"}, "size": {"50"}}, "alpha_name", false},
		query{url.Values{"type": {"byYear"}, "fromYear": {"2018"}, "toYear": {"2021"}, "size": {"50"}}, "by_year", false},
		// DSub sometimes inverts the range -- handler should still accept it
		query{url.Values{"type": {"byYear"}, "fromYear": {"2021"}, "toYear": {"2018"}, "size": {"50"}}, "by_year_inverted", false},
		query{url.Values{"type": {"byGenre"}, "genre": {"Rock"}, "size": {"50"}}, "by_genre_rock", false},
		query{url.Values{"type": {"byGenre"}, "genre": {"Jazz"}, "size": {"50"}}, "by_genre_jazz", false},
		query{url.Values{"type": {"frequent"}, "size": {"50"}}, "frequent", false},
		query{url.Values{"type": {"newest"}, "size": {"50"}}, "newest", false},
		query{url.Values{"type": {"random"}, "size": {"50"}}, "random", true},
		query{url.Values{"type": {"recent"}, "size": {"50"}}, "recent", false},
		query{url.Values{"type": {"starred"}, "size": {"50"}}, "starred_admin", false},
		query{url.Values{"type": {"highest"}, "size": {"50"}}, "highest_admin", false},
		// folder filter stacked on a join-heavy type -- verifies WithAlbumRootDir
		// composes correctly after the type-specific joins
		query{url.Values{"type": {"alphabeticalByArtist"}, "musicFolderId": {"0"}, "size": {"50"}}, "alpha_artist_folder_0", false},
		query{url.Values{"type": {"alphabeticalByArtist"}, "musicFolderId": {"1"}, "size": {"50"}}, "alpha_artist_folder_1", false},
		query{url.Values{"type": {"garbage"}}, "unknown_type", false},
	)
	// alt has divergent stars/ratings -- different output proves user_id scoping
	f.run(t, f.contr.ServeGetAlbumListTwo, f.alt,
		query{url.Values{"type": {"starred"}, "size": {"50"}}, "starred_alt", false},
		query{url.Values{"type": {"highest"}, "size": {"50"}}, "highest_alt", false},
	)
}

func TestSearchThree(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeSearchThree, f.admin,
		query{url.Values{"query": {"album"}}, "q_album", false},
		query{url.Values{"query": {"artist"}}, "q_artist", false},
		query{url.Values{"query": {"track"}}, "q_track", false},
		query{url.Values{"query": {"\"\""}}, "q_empty_all", false},
		// UUID query takes the tag_brainz_id branch instead of fuzzy LIKE
		query{url.Values{"query": {"00000000-0000-0000-0000-0000000000aa"}}, "q_uuid_album", false},
		query{url.Values{"query": {"album"}, "musicFolderId": {"1"}}, "q_album_folder_1", false},
	)
}

func TestGetStarredTwo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetStarredTwo, f.admin,
		query{url.Values{}, "admin", false},
		query{url.Values{"musicFolderId": {"0"}}, "admin_folder_0", false},
		query{url.Values{"musicFolderId": {"1"}}, "admin_folder_1", false},
	)
	f.run(t, f.contr.ServeGetStarredTwo, f.alt,
		query{url.Values{}, "alt", false},
	)
}

func TestGetGenres(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetGenres, f.admin,
		query{url.Values{}, "all", false},
	)
}

func TestGetSongsByGenre(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetSongsByGenre, f.admin,
		query{url.Values{"genre": {"Rock"}, "count": {"50"}}, "rock", false},
		query{url.Values{"genre": {"Jazz"}, "count": {"50"}}, "jazz", false},
		query{url.Values{"genre": {"Pop"}, "count": {"50"}}, "pop", false},
		query{url.Values{"genre": {"Rock"}, "count": {"50"}, "musicFolderId": {"1"}}, "rock_folder_1", false},
	)
}

func TestGetAlbumInfoTwo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetAlbumInfoTwo, f.admin,
		query{url.Values{"id": {f.albumAA.SID().String()}}, "album_aa", false},
	)
}

func TestGetArtistInfoTwo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetArtistInfoTwo, f.admin,
		query{url.Values{"id": {f.artistA.SID().String()}}, "artist_a", false},
		// includeNotPresent surfaces lastfm-similar artists that aren't in the
		// local DB as stubs (LEFT JOIN branch of ArtistWithRolesAndAlbumCount)
		query{url.Values{"id": {f.artistA.SID().String()}, "includeNotPresent": {"true"}}, "artist_a_incl_not_present", false},
	)
}
