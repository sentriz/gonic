package ctrlsubsonic

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

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
