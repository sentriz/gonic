package podcasts

import (
	"os"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

func TestGetMoreRecentEpisodes(t *testing.T) {
	fp := gofeed.NewParser()
	newFile, err := os.Open("testdata/rss.new")
	if err != nil {
		t.Fatal(err)
	}
	newFeed, err := fp.Parse(newFile)
	if err != nil {
		t.Fatal(err)
	}
	after, err := time.Parse(time.RFC1123, "Mon, 27 Jun 2016 06:33:43 +0000")
	if err != nil {
		t.Fatal(err)
	}
	entries := getEntriesAfterDate(newFeed.Items, after)
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}
}
