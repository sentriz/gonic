package ctrlsubsonic

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
)

func TestParseAlbumNFOReview(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name: "jellyfin standard album nfo",
			content: `<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<album>
  <review>An iconic progressive rock concept album.</review>
  <outline>A progressive rock masterpiece.</outline>
  <lockdata>false</lockdata>
  <title>The Dark Side of the Moon</title>
  <year>1973</year>
</album>`,
			want: "An iconic progressive rock concept album.",
		},
		{
			name:    "utf8 bom and multiline crlf preserved",
			content: "\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"utf-8\"?>\r\n<album>\r\n  <review>\r\nFirst paragraph of album background.\r\n\r\nSecond paragraph of track commentary. Read more on official site.\r\n  </review>\r\n</album>",
			want:    "First paragraph of album background.\n\nSecond paragraph of track commentary. Read more on official site.",
		},
		{
			name: "cdata and xml entities",
			content: `<?xml version="1.0" encoding="utf-8"?>
<album>
  <review><![CDATA[Recorded in 1973 & remastered in 2023.
Line two with <tags> intact.]]></review>
</album>`,
			want: "Recorded in 1973 & remastered in 2023.\nLine two with <tags> intact.",
		},
		{
			name: "multiple review tags last non-empty wins",
			content: `<album>
  <review>First review</review>
  <review>Updated review</review>
  <review>   </review>
</album>`,
			want: "Updated review",
		},
		{
			name:    "empty or missing review",
			content: `<album><title>No Review</title><review>   </review></album>`,
			want:    "",
		},
		{
			name:    "wrong root element",
			content: `<artist><name>Pink Floyd</name><review>Wrong root</review></artist>`,
			wantErr: true,
		},
		{
			name:    "malformed xml",
			content: `<album><review>unclosed`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseAlbumNFOReview(strings.NewReader(tt.content))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAlbumNotesFromNFOEdgeCases(t *testing.T) {
	t.Parallel()

	notes, err := albumNotesFromNFO(db.Album{})
	require.NoError(t, err)
	assert.Empty(t, notes)

	tmpDir := t.TempDir()
	notes, err = albumNotesFromNFO(db.Album{RootDir: tmpDir, RightPath: "missing-album"})
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestGetAlbumInfoTwoNFO(t *testing.T) {
	t.Parallel()

	queryAlbumInfo := func(t *testing.T, f *fixture, album db.Album) *spec.AlbumInfo {
		t.Helper()
		body := f.query(t, f.contr.ServeGetAlbumInfoTwo, f.admin, url.Values{
			"id": {album.SID().String()},
		})
		var sub spec.SubsonicResponse
		require.NoError(t, json.Unmarshal([]byte(body), &sub))
		require.Equal(t, "ok", sub.Response.Status)
		require.NotNil(t, sub.Response.AlbumInfo)
		return sub.Response.AlbumInfo
	}

	t.Run("local nfo overrides lastfm notes and skips mb disambiguation while keeping urls and mbid", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)

		require.NoError(t, f.dbc.Model(&db.AlbumInfo{}).
			Where("id=?", f.albumAA.ID).
			Update("music_brainz_disambiguation", "deluxe edition").Error)

		nfoPath := filepath.Join(f.albumAA.AbsPath(), "album.nfo")
		nfoXML := `<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<album>
  <title>album-aa</title>
  <review>Local NFO review paragraph one.

Local NFO review paragraph two.</review>
</album>`
		require.NoError(t, os.WriteFile(nfoPath, []byte(nfoXML), 0o600))

		got := queryAlbumInfo(t, f, f.albumAA)
		assert.Equal(t, "Local NFO review paragraph one.\n\nLocal NFO review paragraph two.", got.Notes)
		assert.Equal(t, "00000000-0000-0000-0000-0000000000aa", got.MusicBrainzID)
		assert.Equal(t, "https://example.invalid/album-aa", got.LastFMURL)

		// Updating album.nfo takes effect immediately without rescanning.
		updatedXML := `<album><review>Updated local NFO album review.</review></album>`
		require.NoError(t, os.WriteFile(nfoPath, []byte(updatedXML), 0o600))
		got = queryAlbumInfo(t, f, f.albumAA)
		assert.Equal(t, "Updated local NFO album review.", got.Notes)

		// Removing album.nfo seamlessly falls back to Last.fm + MusicBrainz disambiguation.
		require.NoError(t, os.Remove(nfoPath))
		got = queryAlbumInfo(t, f, f.albumAA)
		assert.Equal(t, "an album that exists.\n\nDeluxe edition.", got.Notes)
		assert.Equal(t, "https://example.invalid/album-aa", got.LastFMURL)
	})

	t.Run("empty review or malformed nfo falls back to lastfm", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		nfoPath := filepath.Join(f.albumAA.AbsPath(), "album.nfo")

		require.NoError(t, os.WriteFile(nfoPath, []byte(`<album><title>album-aa</title><review>   </review></album>`), 0o600))
		got := queryAlbumInfo(t, f, f.albumAA)
		assert.Equal(t, "an album that exists.", got.Notes)
		assert.Equal(t, "https://example.invalid/album-aa", got.LastFMURL)

		require.NoError(t, os.WriteFile(nfoPath, []byte(`<album><review>broken xml`), 0o600))
		got = queryAlbumInfo(t, f, f.albumAA)
		assert.Equal(t, "an album that exists.", got.Notes)
		assert.Equal(t, "https://example.invalid/album-aa", got.LastFMURL)
	})

	t.Run("works without cached lastfm info or when album tags are missing", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)

		// albumAB has no pre-populated AlbumInfo row and lastfmClient is nil.
		nfoABPath := filepath.Join(f.albumAB.AbsPath(), "album.nfo")
		require.NoError(t, os.WriteFile(nfoABPath, []byte(`<album><review>Offline NFO review for album-ab.</review></album>`), 0o600))
		gotAB := queryAlbumInfo(t, f, f.albumAB)
		assert.Equal(t, "Offline NFO review for album-ab.", gotAB.Notes)
		assert.Equal(t, "00000000-0000-0000-0000-0000000000ab", gotAB.MusicBrainzID)
		assert.Empty(t, gotAB.LastFMURL)

		// Untagged folder album where albumInfoCache.GetOrLookup returns an error ("no metadata to look up").
		untaggedDir := filepath.Join(f.m.TmpDir(), "m-0", "artist-a", "untagged-folder")
		require.NoError(t, os.MkdirAll(untaggedDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(untaggedDir, "album.nfo"), []byte(`<album><review>Folder-only album review.</review></album>`), 0o600))

		untaggedAlbum := db.Album{
			RootDir:     filepath.Join(f.m.TmpDir(), "m-0"),
			LeftPath:    "artist-a/",
			RightPath:   "untagged-folder",
			ParentID:    f.albumAA.ParentID,
			TagBrainzID: "00000000-0000-0000-0000-0000000000ff",
		}
		require.NoError(t, f.dbc.Save(&untaggedAlbum).Error)

		gotUntagged := queryAlbumInfo(t, f, untaggedAlbum)
		assert.Equal(t, "Folder-only album review.", gotUntagged.Notes)
		assert.Equal(t, "00000000-0000-0000-0000-0000000000ff", gotUntagged.MusicBrainzID)
		assert.Empty(t, gotUntagged.LastFMURL)
	})
}
