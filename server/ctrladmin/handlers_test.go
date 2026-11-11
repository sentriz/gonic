package ctrladmin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/deps"
	"go.senan.xyz/gonic/infocache/albuminfocache"
	"go.senan.xyz/gonic/infocache/artistinfocache"
	"go.senan.xyz/gonic/lastfm"
	"go.senan.xyz/gonic/lastfm/mockclient"
	"go.senan.xyz/gonic/server/ctrladmin/adminui"
)

func TestUpdateLastFMLanguage(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, previous, submitted, language string
		refresh                             bool
	}{
		{name: "set language", submitted: "de", language: "de", refresh: true},
		{name: "change language", previous: "de", submitted: "fr", language: "fr", refresh: true},
		{name: "restore default", previous: "de", refresh: true},
		{name: "keep language", previous: "de", submitted: " DE ", language: "de"},
		{name: "keep default"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dbc, err := db.NewMock(deps.DBDriverOptions())
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, dbc.Close()) })
			require.NoError(t, dbc.Migrate(db.MigrationContext{}))
			require.NoError(t, dbc.SetSetting(db.LastFMLanguage, tc.previous))

			artist := db.Artist{Name: "artist"}
			album := db.Album{RightPath: "album", TagAlbumArtist: "artist", TagTitle: "album"}
			require.NoError(t, dbc.Create(&artist).Error)
			require.NoError(t, dbc.Create(&album).Error)
			require.NoError(t, dbc.Create(&db.ArtistInfo{ID: artist.ID, LastFMBiography: "cached biography"}).Error)
			require.NoError(t, dbc.Create(&db.AlbumInfo{ID: album.ID, LastFMNotes: "cached notes"}).Error)

			controller := &Controller{dbc: dbc}
			form := url.Values{"api_key": {"key"}, "secret": {"secret"}, "lang": {tc.submitted}}
			req := httptest.NewRequest(http.MethodPost, "/admin/update_lastfm_api_key_do", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp := controller.ServeUpdateLastFMAPIKeyDo(req)
			require.Empty(t, resp.flashW)
			require.Equal(t, "/admin/home", resp.redirect)

			get := httptest.NewRequest(http.MethodGet, "/admin/update_lastfm_api_key", nil)
			get = get.WithContext(context.WithValue(get.Context(), CtxUser, &db.User{Name: "admin"}))
			page := httptest.NewRecorder()
			render := respHandler(adminui.TemplatesFS, func(path string) string { return path })
			render(controller.ServeUpdateLastFMAPIKey).ServeHTTP(page, get)
			require.Equal(t, http.StatusOK, page.Code)
			require.Contains(t, page.Body.String(), fmt.Sprintf(`id="lang" name="lang" value="%s"`, tc.language))

			var lookups atomic.Int32
			client := lastfm.NewClientCustom("agent", mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
				params := r.URL.Query()
				switch params.Get("method") {
				case "artist.getInfo":
					lookups.Add(1)
					require.Equal(t, tc.language, params.Get("lang"))
					require.Equal(t, tc.language != "", params.Has("lang"))
					fmt.Fprint(w, `<lfm><artist><bio><summary>new biography</summary></bio></artist></lfm>`)
				case "album.getInfo":
					lookups.Add(1)
					require.Equal(t, tc.language, params.Get("lang"))
					require.Equal(t, tc.language != "", params.Has("lang"))
					fmt.Fprint(w, `<lfm><album><wiki><content>new notes</content></wiki></album></lfm>`)
				default:
					fmt.Fprint(w, `<lfm></lfm>`)
				}
			}), func() (string, string, string, error) {
				language, err := dbc.GetSetting(db.LastFMLanguage)
				return "key", "secret", language, err
			})
			artistCache := artistinfocache.New(dbc, client, nil)
			albumCache := albuminfocache.New(dbc, client, nil)
			artistInfo, err := artistCache.GetOrLookup(context.Background(), artist.ID)
			require.NoError(t, err)
			albumInfo, err := albumCache.GetOrLookup(context.Background(), album.ID)
			require.NoError(t, err)
			if !tc.refresh {
				require.Equal(t, int32(0), lookups.Load())
				require.Equal(t, "cached biography", artistInfo.LastFMBiography)
				require.Equal(t, "cached notes", albumInfo.LastFMNotes)
				return
			}
			updatedArtist, err := artistCache.GetOrLookup(context.Background(), artist.ID)
			require.NoError(t, err)
			updatedAlbum, err := albumCache.GetOrLookup(context.Background(), album.ID)
			require.NoError(t, err)
			require.Equal(t, int32(2), lookups.Load())
			require.Equal(t, "new biography", updatedArtist.LastFMBiography)
			require.Equal(t, "new notes", updatedAlbum.LastFMNotes)
		})
	}
}
