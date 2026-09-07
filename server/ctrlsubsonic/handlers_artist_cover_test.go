package ctrlsubsonic

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

func TestArtistCoverFallback(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	require.NoError(t, f.dbc.Save(&db.TrackCredit{TrackID: f.trackAB1.ID, ArtistID: f.artistB.ID, Role: db.RoleComposer}).Error)
	rnd := spec.Render{DB: f.dbc, UserID: f.admin.ID}

	for _, tc := range []struct {
		name     string
		imageURL string
		coverAA  string
		want     *specid.ID
	}{
		{"lastfm", "https://example.invalid/artist.jpg", "", f.artistA.SID()},
		{"album", "", "", f.albumAB.SID()},
		{"newest_album", "", "cover.jpg", f.albumAA.SID()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, f.dbc.Model(&db.ArtistInfo{}).Where("id=?", f.artistA.ID).UpdateColumn("image_url", tc.imageURL).Error)
			require.NoError(t, f.dbc.Model(&db.Album{}).Where("id=?", f.albumAA.ID).
				Updates(map[string]any{"cover": tc.coverAA, "tag_year": 2100}).Error)

			artists, err := spec.ArtistsByTags(rnd, []*db.Artist{&f.artistA, &f.artistB, &f.artistC})
			require.NoError(t, err)
			require.Len(t, artists, 3)
			require.Equal(t, tc.want, artists[0].CoverID)
			require.Equal(t, f.albumAB.SID(), artists[1].CoverID)
			require.Nil(t, artists[2].CoverID)

			_, req := makeHTTPMock(url.Values{"id": {f.artistA.SID().String()}}, f.admin)
			sub := f.contr.ServeGetArtistInfoTwo(req)
			require.Nil(t, sub.Error)
			require.NotNil(t, sub.ArtistInfoTwo)
			if tc.imageURL != "" {
				require.Equal(t, tc.imageURL, sub.ArtistInfoTwo.LargeImageURL)
			} else {
				coverURL, err := url.Parse(sub.ArtistInfoTwo.LargeImageURL)
				require.NoError(t, err)
				require.Equal(t, tc.want.String(), coverURL.Query().Get("id"))
			}
		})
	}
}
