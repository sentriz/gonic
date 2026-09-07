package spec

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/deps"
)

func TestLoadExtrasAcrossChunks(t *testing.T) {
	dbc, err := db.NewMock(deps.DBDriverOptions())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, dbc.Close()) })
	require.NoError(t, dbc.Migrate(db.MigrationContext{}))
	require.NoError(t, dbc.Save(&db.User{ID: 2, Name: "alt", Password: "alt"}).Error)

	genre := &db.Genre{Name: "Rock"}
	require.NoError(t, dbc.Save(genre).Error)
	played := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, id := range []int{1, db.SQLVariableLimit + 1} {
		require.NoError(t, dbc.Save(&db.Artist{ID: id, Name: "artist"}).Error)
		require.NoError(t, dbc.Save(&db.Album{ID: id, RightPath: "album"}).Error)
		require.NoError(t, dbc.Save(&db.Track{ID: id, AlbumID: id, Filename: "track.flac", Length: 120}).Error)
		require.NoError(t, dbc.Save(&db.AlbumCredit{AlbumID: id, ArtistID: id, Role: db.RoleAlbumArtist}).Error)
		require.NoError(t, dbc.Save(&db.TrackCredit{TrackID: id, ArtistID: id, Role: db.RoleArtist}).Error)
		require.NoError(t, dbc.Save(&db.TrackCredit{TrackID: id, ArtistID: id, Role: db.RoleAlbumArtist}).Error)
		require.NoError(t, dbc.Save(&db.TrackRating{TrackID: id, UserID: 1, Rating: 3}).Error)
		require.NoError(t, dbc.Save(&db.TrackRating{TrackID: id, UserID: 2, Rating: 4}).Error)
		require.NoError(t, dbc.Save(&db.TrackPlay{TrackID: id, UserID: 1, Count: 2, Time: played}).Error)
		require.NoError(t, dbc.Save(&db.TrackPlay{TrackID: id, UserID: 2, Count: 9, Time: played}).Error)
		require.NoError(t, dbc.InsertBulkLeftMany("track_genres", []string{"track_id", "genre_id"}, id, []int{genre.ID}))
	}

	keys := make([]int, 0, db.SQLVariableLimit+3)
	for id := 1; id <= db.SQLVariableLimit+1; id++ {
		keys = append(keys, id)
	}
	keys = append(keys, 1, db.SQLVariableLimit+1)

	roles, err := loadArtistRoles(dbc, keys)
	require.NoError(t, err)
	require.Len(t, roles, 2)
	genres, err := loadGenres(dbc, "track_genres", "track_id", keys)
	require.NoError(t, err)
	require.Len(t, genres, 2)
	stats, err := loadAlbumTrackStats(dbc, keys)
	require.NoError(t, err)
	require.Len(t, stats, 2)
	plays, err := loadAlbumPlays(dbc, 1, keys)
	require.NoError(t, err)
	require.Len(t, plays, 2)
	ratings, err := loadAverageRatings(dbc, db.TrackRating{}, "track_id", keys)
	require.NoError(t, err)
	require.Len(t, ratings, 2)

	for _, id := range []int{1, db.SQLVariableLimit + 1} {
		require.Equal(t, []string{db.RoleAlbumArtist, db.RoleArtist}, roles[id])
		require.Len(t, genres[id], 1)
		require.Equal(t, genre.ID, genres[id][0].ID)
		require.Equal(t, 1, stats[id].TrackCount)
		require.Equal(t, 120, stats[id].Duration)
		require.Equal(t, float64(2), plays[id].PlayCount)
		require.True(t, played.Equal(plays[id].PlayTime.Time))
		require.Equal(t, 3.5, ratings[id])
	}
}
