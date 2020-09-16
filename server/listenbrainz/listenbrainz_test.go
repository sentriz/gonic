package listenbrainz

import (
	"testing"

	"go.senan.xyz/gonic/server/db"
)

func TestScrobbleDisabled(t *testing.T) {
	err := Scrobble(false, false, "", "", ScrobbleOptions{})
	if err != nil {
		t.Errorf("expected error, got %s", err)
	}
}

func TestScrobbleTrackNil(t *testing.T) {
	url := "foo"
	token := "bar"
	opts := ScrobbleOptions{
		Track: nil,
	}
	err := Scrobble(true, true, url, token, opts)
	if err == nil {
		t.Errorf("expected error, got %s", err)
	}
}

func TestScrobble(t *testing.T) {
	url := "http://127.0.0.1:0"
	token := "baz"
	track := db.Track{}
	opts := ScrobbleOptions{
		Track:      &track,
		Submission: false,
	}
	err := Scrobble(true, true, url, token, opts)
	if err == nil {
		t.Errorf("expected error, got %s", err)
	}
	opts.UnixTimestampS = 1234567890
	opts.Submission = true
	err = Scrobble(true, true, url, token, opts)
	if err == nil {
		t.Errorf("expected error, got %s", err)
	}
	track.TagTrackNumber = 23
	track.TagBrainzID = "23"
	track.Length = 23
	track.Album = &db.Album{
		TagTitle: "foobar",
	}
	track.Artist = &db.Artist{
		Name: "barfoo",
	}
	err = Scrobble(true, true, url, token, opts)
	if err == nil {
		t.Errorf("expected error, got %s", err)
	}
}
