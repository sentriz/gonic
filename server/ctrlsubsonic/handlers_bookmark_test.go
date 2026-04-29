package ctrlsubsonic

import (
	"net/url"
	"testing"
	"testing/synctest"
)

func TestGetBookmarks(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.seq = true

	f.run(t, f.contr.ServeGetBookmarks, f.admin,
		query{url.Values{}, "empty", false},
	)

	// bubble pins gorm's auto-stamped CreatedAt/UpdatedAt
	synctest.Test(t, func(*testing.T) {
		f.query(t, f.contr.ServeCreateBookmark, f.admin, url.Values{
			"id":       {f.trackAB1.SID().String()},
			"position": {"42000"},
			"comment":  {"halfway through"},
		})
		f.query(t, f.contr.ServeCreateBookmark, f.alt, url.Values{
			"id":       {f.trackVA0.SID().String()},
			"position": {"5000"},
		})
	})

	f.run(t, f.contr.ServeGetBookmarks, f.admin,
		query{url.Values{}, "admin_after_create", false},
	)
	f.run(t, f.contr.ServeGetBookmarks, f.alt,
		query{url.Values{}, "alt_after_create", false},
	)

	// re-creating with the same id exercises FirstOrCreate's update branch
	synctest.Test(t, func(*testing.T) {
		f.query(t, f.contr.ServeCreateBookmark, f.admin, url.Values{
			"id":       {f.trackAB1.SID().String()},
			"position": {"99000"},
			"comment":  {"almost done"},
		})
	})
	f.run(t, f.contr.ServeGetBookmarks, f.admin,
		query{url.Values{}, "admin_after_update", false},
	)

	// alt's bookmark must survive admin's delete -- exercises user_id scoping
	f.query(t, f.contr.ServeDeleteBookmark, f.admin, url.Values{
		"id": {f.trackAB1.SID().String()},
	})
	f.run(t, f.contr.ServeGetBookmarks, f.admin,
		query{url.Values{}, "admin_after_delete", false},
	)
	f.run(t, f.contr.ServeGetBookmarks, f.alt,
		query{url.Values{}, "alt_after_admin_delete", false},
	)
}
