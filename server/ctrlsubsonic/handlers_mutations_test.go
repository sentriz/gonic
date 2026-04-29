package ctrlsubsonic

import (
	"net/url"
	"testing"
	"testing/synctest"
)

func TestStarUnstar(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	// time.Now() is stampted, testing/synctest bubble needed
	synctest.Test(t, func(*testing.T) {
		f.query(t, f.contr.ServeStar, f.admin, url.Values{
			"albumId":  {f.albumBA.SID().String()},
			"artistId": {f.artistB.SID().String()},
		})
		f.query(t, f.contr.ServeUnstar, f.admin, url.Values{
			"albumId": {f.albumAA.SID().String()},
		})
	})

	f.run(t, f.contr.ServeGetStarredTwo, f.admin,
		query{url.Values{}, "after_star_unstar", false},
	)
}

func TestSetRating(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	f.query(t, f.contr.ServeSetRating, f.admin, url.Values{
		"id":     {f.albumVA.SID().String()},
		"rating": {"2"},
	})
	f.query(t, f.contr.ServeSetRating, f.admin, url.Values{
		"id":     {f.albumVA.SID().String()},
		"rating": {"5"},
	})
	f.query(t, f.contr.ServeSetRating, f.admin, url.Values{
		"id":     {f.albumAA.SID().String()},
		"rating": {"0"},
	})
	f.query(t, f.contr.ServeSetRating, f.admin, url.Values{
		"id":     {f.trackVA0.SID().String()},
		"rating": {"4"},
	})
	f.query(t, f.contr.ServeSetRating, f.admin, url.Values{
		"id":     {f.artistB.SID().String()},
		"rating": {"3"},
	})

	f.run(t, f.contr.ServeGetAlbum, f.admin,
		query{url.Values{"id": {f.albumVA.SID().String()}}, "set_rating_album_va", false},
		query{url.Values{"id": {f.albumAA.SID().String()}}, "set_rating_album_aa_cleared", false},
	)
	f.run(t, f.contr.ServeGetArtist, f.admin,
		query{url.Values{"id": {f.artistB.SID().String()}}, "set_rating_artist_b", false},
	)
}
