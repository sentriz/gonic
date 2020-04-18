package scanner

import (
	"io/ioutil"
	"log"
	"testing"

	_ "github.com/jinzhu/gorm/dialects/sqlite"

	"go.senan.xyz/gonic/db"
)

var testScanner *Scanner

func init() {
	db, err := db.NewMock()
	if err != nil {
		log.Fatalf("error opening database: %v\n", err)
	}
	// benchmarks aren't real code are they? >:)
	// here is an absolute path to my music directory
	testScanner = New("/home/senan/music", db)
	log.SetOutput(ioutil.Discard)
}

func resetTables(db *db.DB) {
	tx := db.Begin()
	defer tx.Commit()
	tx.Exec("delete from tracks")
	tx.Exec("delete from artists")
	tx.Exec("delete from albums")
}

func resetTablesPause(db *db.DB, b *testing.B) {
	b.StopTimer()
	defer b.StartTimer()
	resetTables(db)
}

func BenchmarkScanFresh(b *testing.B) {
	for n := 0; n < b.N; n++ {
		resetTablesPause(testScanner.db, b)
		testScanner.Start()
	}
}

func BenchmarkScanIncremental(b *testing.B) {
	// do a full scan and reset
	testScanner.Start()
	b.ResetTimer()
	// do the inc scans
	for n := 0; n < b.N; n++ {
		testScanner.Start()
	}
}

// RESULTS fresh
// 20 times / 1.436
// 20 times / 1.39

// RESULTS inc
// 100 times / 1.86
// 100 times / 1.9
// 100 times / 1.5
// 100 times / 1.48
