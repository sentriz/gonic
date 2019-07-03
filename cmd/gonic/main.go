package main

import (
	"flag"
	"log"
	"os"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/peterbourgon/ff"

	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/server"
)

const (
	programName = "gonic"
	programVar  = "GONIC"
)

func main() {
	set := flag.NewFlagSet(programName, flag.ExitOnError)
	listenAddr := set.String(
		"listen-addr", "0.0.0.0:6969",
		"listen address (optional)")
	musicPath := set.String(
		"music-path", "",
		"path to music")
	dbPath := set.String(
		"db-path", "gonic.db",
		"path to database (optional)")
	_ = set.String(
		"config-path", "",
		"path to config (optional)")
	err := ff.Parse(set, os.Args[1:],
		ff.WithConfigFileFlag("config-path"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix(programVar),
	)
	if err != nil {
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
	s := server.New(
		db,
		*musicPath,
		*listenAddr,
		"server/assets/",
	)
	err = s.SetupAdmin()
	if err != nil {
		log.Fatalf("error setting up admin routes: %v\n", err)
	}
	err = s.SetupSubsonic()
	if err != nil {
		log.Fatalf("error setting up subsonic routes: %v\n", err)
	}
	log.Printf("starting server at %s", *listenAddr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatalf("error starting server: %v\n", err)
	}
}
