package db

import (
	"io"
	"log"
	"math/rand"
	"os"
	"testing"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func TestGetSetting(t *testing.T) {
	t.Parallel()

	key := SettingKey(randKey())
	value := "howdy"

	testDB, err := NewMock()
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

func randKey() string {
	letters := []rune("abcdef0123456789")
	b := make([]rune, 16)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
