package main

import (
	"fmt"
	"log"
	"os"

	_ "gorm.io/driver/sqlite"

	"go.senan.xyz/gonic/server/scanner/tags"
)

func main() {
	t, err := tags.New(os.Args[1])
	if err != nil {
		log.Fatalf("error reading: %v", err)
	}
	fmt.Println("artist", t.Album())
	fmt.Println("aartist", t.AlbumArtist())
	fmt.Println("len", t.Length())
	fmt.Println("br", t.Bitrate())
}
