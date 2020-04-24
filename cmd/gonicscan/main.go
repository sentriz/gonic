package main

import (
	"flag"
	"log"
	"os"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/peterbourgon/ff"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scanner"
	"go.senan.xyz/gonic/version"
)

func main() {
	set := flag.NewFlagSet(version.NAME_SCAN, flag.ExitOnError)
	musicPath := set.String("music-path", "", "path to music")
	dbPath := set.String("db-path", "gonic.db", "path to database (optional)")
	fullScan := set.Bool("full-scan", false, "ignore file modtimes while scanning (optional)")
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
	s := scanner.New(*musicPath, db)
	var scan func() error
	switch {
	case *fullScan:
		scan = s.StartFull
	default:
		scan = s.StartInc
	}
	if err := scan(); err != nil {
		log.Fatalf("error during scan: %v\n", err)
	}
}
