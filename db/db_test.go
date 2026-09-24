package db

import (
	"io"
	"log"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"go.senan.xyz/gonic/deps"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func TestGetSetting(t *testing.T) {
	t.Parallel()

	key := SettingKey(randKey())
	value := "howdy"

	testDB, err := NewMock(deps.DBDriverOptions())
	if err != nil {
		t.Fatalf("error creating db: %v", err)
	}
	if err := testDB.Migrate(MigrationContext{}); err != nil {
		t.Fatalf("error migrating db: %v", err)
	}

	require.NoError(t, testDB.SetSetting(key, value))

	actual, err := testDB.GetSetting(key)
	require.NoError(t, err)
	require.Equal(t, value, actual)

	require.NoError(t, testDB.SetSetting(key, value))
	actual, err = testDB.GetSetting(key)
	require.NoError(t, err)
	require.Equal(t, value, actual)
}

func TestPreloadManyRows(t *testing.T) {
	t.Parallel()

	testDB, err := NewMock(deps.DBDriverOptions())
	require.NoError(t, err)
	require.NoError(t, testDB.Migrate(MigrationContext{}))

	const n = 40_000
	require.NoError(t, testDB.Exec(`
		WITH RECURSIVE c(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM c WHERE i<?)
		INSERT INTO artists (id, name) SELECT i, 'artist '||i FROM c`, n).Error)
	require.NoError(t, testDB.Exec(`INSERT INTO artist_stars (user_id, artist_id, star_date) SELECT 1, id, datetime() FROM artists WHERE id%50=0`).Error)
	require.NoError(t, testDB.Exec(`INSERT INTO artist_infos (id, image_url) SELECT id, 'x' FROM artists WHERE id%3=0`).Error)

	var artists []*Artist
	require.NoError(t, testDB.
		Preload("ArtistStar", "user_id=?", 1).
		Preload("ArtistRating", "user_id=?", 1).
		Preload("Info").
		Order("id").
		Find(&artists).
		Error)

	require.Len(t, artists, n)
	for _, a := range artists {
		require.Equal(t, a.ID%50 == 0, a.ArtistStar != nil, "artist %d star", a.ID)
		require.Nil(t, a.ArtistRating, "artist %d rating", a.ID)
		require.Equal(t, a.ID%3 == 0, a.Info != nil, "artist %d info", a.ID)
	}
}

func randKey() string {
	letters := []rune("abcdef0123456789")
	b := make([]rune, 16)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
