package db

import (
	"log"
	"math/rand"
	"testing"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var testDB *DB

func init() {
	var err error
	testDB, err = NewMock()
	if err != nil {
		log.Fatalf("error opening database: %v\n", err)
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

func TestGetSetting(t *testing.T) {
	key := randKey()
	// new key
	expected := "hello"
	testDB.SetSetting(key, expected)
	actual := testDB.GetSetting(key)
	if actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}
	// existing key
	expected = "howdy"
	testDB.SetSetting(key, expected)
	actual = testDB.GetSetting(key)
	if actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}
}
