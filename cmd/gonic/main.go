package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/peterbourgon/ff"

	"senan.xyz/g/gonic/db"
	"senan.xyz/g/gonic/server"
	"senan.xyz/g/gonic/version"
)

func main() {
	set := flag.NewFlagSet(version.NAME, flag.ExitOnError)
	listenAddr := set.String("listen-addr", "0.0.0.0:4747", "listen address (optional)")
	musicPath := set.String("music-path", "", "path to music")
	dbPath := set.String("db-path", "gonic.db", "path to database (optional)")
	scanInterval := set.Int("scan-interval", 0, "interval (in minutes) to automatically scan music (optional)")
	proxyPrefix := set.String("proxy-prefix", "", "url path prefix to use if behind proxy. eg '/gonic' (optional)")
	_ = set.String("config-path", "", "path to config (optional)")
	showVersion := set.Bool("version", false, "show gonic version")
	if err := ff.Parse(set, os.Args[1:],
		ff.WithConfigFileFlag("config-path"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix(version.NAME_UPPER),
	); err != nil {
		log.Fatalf("error parsing args: %v\n", err)
	}
	if *showVersion {
		fmt.Println(version.VERSION)
		os.Exit(0)
	}
	if _, err := os.Stat(*musicPath); os.IsNotExist(err) {
		log.Fatal("please provide a valid music directory")
	}
	db, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("error opening database: %v\n", err)
	}
	defer db.Close()
	proxyPrefixExpr := regexp.MustCompile(`^\/*(.*?)\/*$`)
	*proxyPrefix = proxyPrefixExpr.ReplaceAllString(*proxyPrefix, `/$1`)
	serverOptions := server.Options{
		DB:           db,
		MusicPath:    *musicPath,
		ListenAddr:   *listenAddr,
		ScanInterval: time.Duration(*scanInterval) * time.Minute,
		ProxyPrefix:  *proxyPrefix,
	}
	log.Printf("using opts %+v\n", serverOptions)
	s := server.New(serverOptions)
	log.Printf("starting server at %s", *listenAddr)
	if err := s.Start(); err != nil {
		log.Fatalf("error starting server: %v\n", err)
	}
}
