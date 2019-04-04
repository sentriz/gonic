package tags

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestRead(t *testing.T) {
	start := time.Now()
	cwd, _ := os.Getwd()
	data, err := Read(
		fmt.Sprintf("%s/static/test_flac", cwd),
	)
	if err != nil {
		t.Errorf("when reading tags: %v\n", err)
		return
	}
	length := data.Length()
	if length != 160.4 {
		t.Errorf("expected length to be `160.4`, got %f", length)
	}
	bitrate := data.Bitrate()
	if bitrate != 815694 {
		t.Errorf("expected bitrate to be `815694`, got %d", bitrate)
	}
	format := data.Format()
	if format != "flac" {
		t.Errorf("expected format to be `flac`, got %s", format)
	}
	fmt.Println(data.Title())
	fmt.Println(data.Album())
	fmt.Println(data.AlbumArtist())
	fmt.Println(data.Year())
	fmt.Printf("it's been %s\n", time.Since(start))
}
