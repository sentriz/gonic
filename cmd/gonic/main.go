package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/oklog/run"
	"github.com/peterbourgon/ff"

	"go.senan.xyz/gonic/server"
	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/version"
)

func main() {
	set := flag.NewFlagSet(version.NAME, flag.ExitOnError)
	listenAddr := set.String("listen-addr", "0.0.0.0:4747", "listen address (optional)")
	musicPath := set.String("music-path", "", "path to music")
	cachePath := set.String("cache-path", "/tmp/gonic_cache", "path to cache")
	dbPath := set.String("db-path", "gonic.db", "path to database (optional)")
	scanInterval := set.Int("scan-interval", 0, "interval (in minutes) to automatically scan music (optional)")
	jukeboxEnabled := set.Bool("jukebox-enabled", false, "whether the subsonic jukebox api should be enabled (optional)")
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
	//
	if *showVersion {
		fmt.Println(version.VERSION)
		os.Exit(0)
	}
	log.Printf("starting gonic %s\n", version.VERSION)
	log.Printf("provided config\n")
	set.VisitAll(func(f *flag.Flag) {
		log.Printf("    %-15s %s\n", f.Name, f.Value)
	})
	if _, err := os.Stat(*musicPath); os.IsNotExist(err) {
		log.Fatal("please provide a valid music directory")
	}
	if _, err := os.Stat(*cachePath); os.IsNotExist(err) {
		if err := os.MkdirAll(*cachePath, os.ModePerm); err != nil {
			log.Fatalf("couldn't create cache path: %v\n", err)
		}
	}
	db, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("error opening database: %v\n", err)
	}
	defer db.Close()
	//
	proxyPrefixExpr := regexp.MustCompile(`^\/*(.*?)\/*$`)
	*proxyPrefix = proxyPrefixExpr.ReplaceAllString(*proxyPrefix, `/$1`)
	//
	server := server.New(server.Options{
		DB:          db,
		MusicPath:   *musicPath,
		CachePath:   *cachePath,
		ProxyPrefix: *proxyPrefix,
	})
	var g run.Group
	g.Add(server.StartHTTP(*listenAddr))
	if *scanInterval > 0 {
		tickerDur := time.Duration(*scanInterval) * time.Minute
		g.Add(server.StartScanTicker(tickerDur))
	}
	if *jukeboxEnabled {
		g.Add(server.StartJukebox())
	}
	if err := g.Run(); err != nil {
		log.Printf("error in job: %v", err)
	}
}
