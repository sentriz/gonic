package main

import (
	"flag"
	"log"
	"os"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/peterbourgon/ff"

	"github.com/sentriz/gonic/scanner"
)

const (
	programName = "gonic"
)

func main() {
	set := flag.NewFlagSet(programName, flag.ExitOnError)
	musicPath := set.String(
		"music-path", "",
		"path to music")
	dbPath := set.String(
		"db-path", "gonic.db",
		"path to database (optional)")
	err := ff.Parse(set, os.Args[1:])
	if err != nil {
		log.Fatalf("error parsing args: %v\n", err)
	}
	if _, err := os.Stat(*musicPath); os.IsNotExist(err) {
		log.Fatal("please provide a valid music directory")
	}
	db, err := gorm.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("error opening database: %v\n", err)
	}
	db.SetLogger(log.New(os.Stdout, "gorm ", 0))
	db.Exec("PRAGMA foreign_keys = ON")
	s := scanner.New(
		db,
		*musicPath,
	)
	if err := s.MigrateDB(); err != nil {
		log.Fatalf("error migrating: %v\n", err)
	}
	if err := s.Start(); err != nil {
		log.Fatalf("error starting scanner: %v\n", err)
	}
}
