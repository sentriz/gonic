package ctrlsubsonic

import (
	"net/url"
	"testing"
)

// these handlers all return TrackChild lists built from spec row types, so
// their golden files are what catches derived data (stars, ratings, play
// counts, average ratings) going missing.

func TestGetSong(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetSong, f.admin,
		query{url.Values{"id": {f.trackAB1.SID().String()}}, "track_ab1_rich_tags", false},
		query{url.Values{"id": {f.trackVA0.SID().String()}}, "track_va0_compilation", false},
	)
	// alt rates and stars differently, so a user-scoped load shows up here
	f.run(t, f.contr.ServeGetSong, f.alt,
		query{url.Values{"id": {f.trackAB1.SID().String()}}, "track_ab1_alt", false},
	)
}

func TestGetRandomSongs(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	// size is well above the fixture's track count, so the *set* is every
	// track and only the order is random. compared as a set.
	f.run(t, f.contr.ServeGetRandomSongs, f.admin,
		query{url.Values{"size": {"100"}}, "all", true},
		query{url.Values{"size": {"100"}, "genre": {"Rock"}}, "genre_rock", true},
		query{url.Values{"size": {"100"}, "musicFolderId": {"1"}}, "folder_1", true},
		query{url.Values{"size": {"100"}, "fromYear": {"2018"}, "toYear": {"2019"}}, "year_range", true},
	)
}

func TestGetTopSongs(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	// artist-a's lastfm top tracks are seeded in the fixture cache
	f.run(t, f.contr.ServeGetTopSongs, f.admin,
		query{url.Values{"artist": {"artist-a"}}, "artist_a", false},
		query{url.Values{"artist": {"artist-a"}, "count": {"1"}}, "artist_a_count_1", false},
		query{url.Values{"artist": {"artist-b"}}, "artist_b_no_top_tracks", false},
	)
}

func TestGetSimilarSongsTwo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.run(t, f.contr.ServeGetSimilarSongsTwo, f.admin,
		query{url.Values{"id": {f.artistA.SID().String()}, "count": {"50"}}, "artist_a", true},
	)
}

func TestGetPlayQueue(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	// the queue mixes a track and a podcast episode. the episode's parent must
	// be the podcast, which only happens if Podcast is preloaded
	f.run(t, f.contr.ServeGetPlayQueue, f.admin,
		query{url.Values{}, "admin_mixed_entries", false},
	)
	// alt has no saved queue
	f.run(t, f.contr.ServeGetPlayQueue, f.alt,
		query{url.Values{}, "alt_empty", false},
	)
}

func TestJukeboxNotEnabled(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	// the fixture has no jukebox, so this only pins the guard. the entry
	// loading below it has no golden coverage -- see the note in the test.
	f.run(t, f.contr.ServeJukebox, f.admin,
		query{url.Values{"action": {"get"}}, "not_enabled", false},
	)
}
