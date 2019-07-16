package main

import (
	"encoding/gob"
	"flag"
	"log"
	"os"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/peterbourgon/ff"

	"senan.xyz/g/gonic/db"
	"senan.xyz/g/gonic/server"
	"senan.xyz/g/gonic/server/handler"
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
	if err := ff.Parse(set, os.Args[1:],
		ff.WithConfigFileFlag("config-path"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix(programVar),
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
	gob.Register(&handler.Flash{})
	s := server.New(
		db,
		*musicPath,
		*listenAddr,
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
