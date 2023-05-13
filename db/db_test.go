package db

import (
	"io"
	"log"
	"math/rand"
	"os"
	"testing"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/stretchr/testify/assert"
)

func randKey() string {
	letters := []rune("abcdef0123456789")
	b := make([]rune, 16)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func TestGetSetting(t *testing.T) {
	key := randKey()
	value := "howdy"

	assert := assert.New(t)

	testDB, err := NewMock()
	if err != nil {
		t.Fatalf("error creating db: %v", err)
	}
	if err := testDB.Migrate(MigrationContext{}); err != nil {
		t.Fatalf("error migrating db: %v", err)
	}

	assert.NoError(testDB.SetSetting(key, value))

	actual, err := testDB.GetSetting(key)
	assert.NoError(err)
	assert.Equal(actual, value)

	assert.NoError(testDB.SetSetting(key, value))
	actual, err = testDB.GetSetting(key)
	assert.NoError(err)
	assert.Equal(actual, value)
}

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}
