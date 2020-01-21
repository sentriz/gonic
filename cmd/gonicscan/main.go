package main

import (
	"flag"
	"log"
	"os"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/peterbourgon/ff"

	"senan.xyz/g/gonic/db"
	"senan.xyz/g/gonic/scanner"
	"senan.xyz/g/gonic/version"
)

func main() {
	set := flag.NewFlagSet(version.NAME_SCAN, flag.ExitOnError)
	musicPath := set.String(
		"music-path", "",
		"path to music")
	dbPath := set.String(
		"db-path", "gonic.db",
		"path to database (optional)")
	if err := ff.Parse(set, os.Args[1:],
		ff.WithEnvVarPrefix(version.NAME_UPPER),
	); err != nil {
		log.Fatalf("error parsing args: %v\n", err)
	}
	if _, err := os.Stat(*musicPath); os.IsNotExist(err) {
		log.Fatal("please provide a valid music directory")
	}
	db, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("error opening database: %v\n", err)
	}
	defer db.Close()
	s := scanner.New(
		db,
		*musicPath,
	)
	if err := s.Start(); err != nil {
		log.Fatalf("error starting scanner: %v\n", err)
	}
}
