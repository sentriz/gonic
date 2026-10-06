package ctrlsubsonic

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.senan.xyz/gonic/db"
)

func TestBuildFuzzy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		query string
		want  string
	}{
		// plain substring search — unchanged behaviour
		{"album", "%album%"},
		{"artist", "%artist%"},
		{"track", "%track%"},
		// multi-word collapses spaces into % separators
		{"foo bar", "%foo%bar%"},
		// trailing * → prefix wildcard (iSub and similar clients)
		{"Abigail*", "abigail%"},
		{"album*", "album%"},
		{"artist*", "artist%"},
		// multi-word with trailing * — words joined, then prefix
		{"foo bar*", "foo%bar%"},
		// bare * is an empty prefix (matches everything)
		{"*", "%"},
		// no * in the middle — treated as literal (substring match)
		{"al*bum", "%al*bum%"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, buildFuzzy(tt.query))
		})
	}
}

func TestGetNowPlaying(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.seq = true

	// fixture plays are from 2020, all outside the one hour window
	f.run(t, f.contr.ServeGetNowPlaying, f.admin,
		query{url.Values{}, "empty", false},
	)

	alt2 := &db.User{Name: "alt2"}
	require.NoError(t, f.dbc.Where("name=?", "alt2").First(alt2).Error)

	// scrobble tracks admin never played, so the golden file has no admin
	// "played" timestamps stamped with time.Now()
	f.query(t, f.contr.ServeScrobble, f.alt, url.Values{
		"id":   {f.trackVA0.SID().String()},
		"time": {fmt.Sprint(time.Now().Add(-5 * time.Minute).UnixMilli())},
	})
	f.query(t, f.contr.ServeScrobble, alt2, url.Values{
		"id":   {f.trackAB1.SID().String()},
		"time": {fmt.Sprint(time.Now().Add(-30 * time.Minute).UnixMilli())},
	})

	f.run(t, f.contr.ServeGetNowPlaying, f.admin,
		query{url.Values{}, "recent_plays", false},
	)
}
