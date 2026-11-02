package ctrlsubsonic

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scanner"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
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
		"time": {fmt.Sprint(time.Now().Add(-20 * time.Minute).UnixMilli())},
	})
	f.query(t, f.contr.ServeScrobble, f.alt, url.Values{
		"id":   {f.trackAB1.SID().String()},
		"time": {fmt.Sprint(time.Now().Add(-10 * time.Minute).UnixMilli())},
	})

	// replaying a track updates its existing play, which has a lower id than the track after it
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

func TestStartScan(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		admin        bool
		scanNonAdmin bool
		wantAllowed  bool
	}{
		{"admin", true, false, true},
		{"non admin", false, false, false},
		{"non admin enabled", false, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			f.contr.scanNonAdmin = tt.scanNonAdmin
			f.contr.scanner = scanner.New(nil, f.dbc, nil, nil, "", false, nil)
			require.True(t, f.contr.scanner.StartScanning())

			user := f.alt
			if tt.admin {
				user = f.admin
			}
			body := f.query(t, f.contr.ServeStartScan, user, url.Values{})
			var sub spec.SubsonicResponse
			require.NoError(t, json.Unmarshal([]byte(body), &sub))

			if !tt.wantAllowed {
				require.Equal(t, "failed", sub.Response.Status)
				require.NotNil(t, sub.Response.Error)
				require.Equal(t, 50, sub.Response.Error.Code)
				return
			}
			require.Equal(t, "ok", sub.Response.Status)
			require.NotNil(t, sub.Response.ScanStatus)
			require.True(t, sub.Response.ScanStatus.Scanning)
		})
	}
}

func TestScrobbleNowPlayingNotCounted(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	f.query(t, f.contr.ServeScrobble, f.alt, url.Values{
		"id":         {f.trackVA0.SID().String()},
		"submission": {"false"},
	})
	f.query(t, f.contr.ServeScrobble, f.alt, url.Values{
		"id": {f.trackVA0.SID().String()},
	})

	var play db.TrackPlay
	require.NoError(t, f.dbc.Where("user_id=? AND track_id=?", f.alt.ID, f.trackVA0.ID).First(&play).Error)
	require.Equal(t, 1.0, play.Count)
	require.Equal(t, f.trackVA0.Length, play.Length)
}
