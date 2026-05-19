package ctrlsubsonic

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	playlistp "go.senan.xyz/gonic/playlist"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
)

// pinPlaylistTimes pins file mtimes -- getPlaylists reads them as UpdatedAt.
func pinPlaylistTimes(t *testing.T, f *fixture) {
	t.Helper()
	stable := time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC)
	paths, err := f.contr.playlistStore.List()
	if err != nil {
		t.Fatalf("list playlists: %v", err)
	}
	for _, p := range paths {
		abs := filepath.Join(f.contr.playlistStore.BasePath(), p)
		if err := os.Chtimes(abs, stable, stable); err != nil {
			t.Fatalf("chtimes playlist: %v", err)
		}
	}
}

func TestGetPlaylists(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	pinPlaylistTimes(t, f)
	f.run(t, f.contr.ServeGetPlaylists, f.admin,
		query{url.Values{}, "admin", false},
	)
	f.run(t, f.contr.ServeGetPlaylists, f.alt,
		query{url.Values{}, "alt_sees_public", false},
	)
}

func TestGetPlaylist(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	pinPlaylistTimes(t, f)
	f.run(t, f.contr.ServeGetPlaylist, f.admin,
		query{url.Values{"id": {f.sharedPlaylistID()}}, "shared", false},
	)
}

func TestCreateOrUpdatePlaylist(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	// the response id is time-based, so we copy the result to a stable path
	// and golden-test getPlaylist against that instead.
	body := f.query(t, f.contr.ServeCreateOrUpdatePlaylist, f.admin, url.Values{
		"name":   {"new-playlist"},
		"songId": {f.trackAB1.SID().String(), f.trackVA0.SID().String()},
	})
	var sub spec.SubsonicResponse
	if err := json.Unmarshal([]byte(body), &sub); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sub.Response.Status != "ok" || sub.Response.Playlist == nil {
		t.Fatalf("unexpected response: %s", body)
	}

	createdID := sub.Response.Playlist.ID
	createdPath := playlistIDDecode(createdID)
	created, err := f.contr.playlistStore.Read(createdPath)
	if err != nil {
		t.Fatalf("read created: %v", err)
	}
	_ = f.contr.playlistStore.Delete(createdPath)
	stablePath := filepath.Join("1", "created.m3u")
	created.UpdatedAt = time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC)
	if err := f.contr.playlistStore.Write(stablePath, created); err != nil {
		t.Fatalf("write: %v", err)
	}
	pinPlaylistTimes(t, f)

	stableID := playlistIDEncode(stablePath).String()
	f.run(t, f.contr.ServeGetPlaylist, f.admin,
		query{url.Values{"id": {stableID}}, "after_create", false},
	)
}

func TestUpdatePlaylist(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	f.query(t, f.contr.ServeUpdatePlaylist, f.admin, url.Values{
		"id":                {f.sharedPlaylistID()},
		"name":              {"updated name"},
		"comment":           {"updated comment"},
		"public":            {"false"},
		"songIndexToRemove": {"0"},
		"songIdToAdd":       {f.trackVA0.SID().String()},
	})
	pinPlaylistTimes(t, f)
	f.run(t, f.contr.ServeGetPlaylist, f.admin,
		query{url.Values{"id": {f.sharedPlaylistID()}}, "after_update", false},
	)
}

func TestDeletePlaylist(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	f.query(t, f.contr.ServeDeletePlaylist, f.admin, url.Values{
		"id": {f.sharedPlaylistID()},
	})
	f.run(t, f.contr.ServeGetPlaylists, f.admin,
		query{url.Values{}, "after_delete", false},
	)
}

func TestGetPlaylistDeniesOtherUsersPrivate(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	privateID := writePrivatePlaylist(t, f)

	body := f.query(t, f.contr.ServeGetPlaylist, f.alt, url.Values{
		"id": {privateID},
	})
	var sub spec.SubsonicResponse
	if err := json.Unmarshal([]byte(body), &sub); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sub.Response.Status != "failed" || sub.Response.Error == nil || sub.Response.Error.Code != 50 {
		t.Fatalf("expected error 50, got: %s", body)
	}
}

func TestDeletePlaylistDeniesOtherUsers(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	body := f.query(t, f.contr.ServeDeletePlaylist, f.alt, url.Values{
		"id": {f.sharedPlaylistID()},
	})
	var sub spec.SubsonicResponse
	if err := json.Unmarshal([]byte(body), &sub); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sub.Response.Status != "failed" || sub.Response.Error == nil || sub.Response.Error.Code != 50 {
		t.Fatalf("expected error 50, got: %s", body)
	}
	if _, err := f.contr.playlistStore.Read(filepath.Join("1", "shared.m3u")); err != nil {
		t.Fatalf("playlist was deleted despite auth failure: %v", err)
	}
}

func TestPlaylistTraversalDenied(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	writePrivatePlaylist(t, f)

	encode := func(parts ...string) string {
		return playlistIDEncode(filepath.Join(parts...)).String()
	}
	altID := fmt.Sprint(f.alt.ID)

	cases := []struct {
		name    string
		handler handlerSubsonic
		id      string
	}{
		{"get_cross_user", f.contr.ServeGetPlaylist, encode(altID, "..", "1", "private.m3u")},
		{"delete_cross_user", f.contr.ServeDeletePlaylist, encode(altID, "..", "1", "shared.m3u")},
		{"create_escapes_base", f.contr.ServeCreateOrUpdatePlaylist, encode("..", "injected.m3u")},
	}
	for _, tc := range cases {
		body := f.query(t, tc.handler, f.alt, url.Values{"id": {tc.id}, "name": {"x"}})
		var sub spec.SubsonicResponse
		if err := json.Unmarshal([]byte(body), &sub); err != nil {
			t.Fatalf("%s: unmarshal: %v", tc.name, err)
		}
		if sub.Response.Status != "failed" || sub.Response.Error == nil {
			t.Fatalf("%s: expected failure, got: %s", tc.name, body)
		}
	}

	if _, err := f.contr.playlistStore.Read(filepath.Join("1", "shared.m3u")); err != nil {
		t.Fatalf("shared playlist deleted via traversal: %v", err)
	}
	escaped := filepath.Join(f.contr.playlistStore.BasePath(), "..", "injected.m3u")
	if _, err := os.Stat(escaped); !os.IsNotExist(err) {
		t.Fatalf("file written outside playlists dir: stat err=%v", err)
	}
}

func writePrivatePlaylist(t *testing.T, f *fixture) string {
	t.Helper()
	relPath := filepath.Join("1", "private.m3u")
	err := f.contr.playlistStore.Write(relPath, &playlistp.Playlist{
		UserID:    f.admin.ID,
		UpdatedAt: time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC),
		Name:      "private playlist",
		IsPublic:  false,
	})
	if err != nil {
		t.Fatalf("write private playlist: %v", err)
	}
	return playlistIDEncode(relPath).String()
}
