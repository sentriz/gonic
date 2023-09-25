package podcasts

import (
	"bytes"
	_ "embed"
	"path/filepath"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/mockfs"
)

//go:embed testdata/rss.new
var testRSS []byte

func TestPodcastsAndEpisodesWithSameName(t *testing.T) {
	t.Parallel()

	t.Skip("requires network access")

	m := mockfs.New(t)

	base := t.TempDir()
	podcasts := New(m.DB(), base, m.TagReader())

	fp := gofeed.NewParser()
	newFeed, err := fp.Parse(bytes.NewReader(testRSS))
	if err != nil {
		t.Fatalf("parse test data: %v", err)
	}

	podcast, err := podcasts.AddNewPodcast("file://testdata/rss.new", newFeed)
	require.NoError(t, err)

	require.Equal(t, podcast.RootDir, filepath.Join(base, "InternetBox"))

	podcast, err = podcasts.AddNewPodcast("file://testdata/rss.new", newFeed)
	require.NoError(t, err)

	// check we made a unique podcast name
	require.Equal(t, podcast.RootDir, filepath.Join(base, "InternetBox (1)"))

	podcastEpisodes, err := podcasts.GetNewestPodcastEpisodes(10)
	require.NoError(t, err)
	require.Greater(t, len(podcastEpisodes), 0)

	var pe []*db.PodcastEpisode
	require.NoError(t, m.DB().Order("id").Find(&pe, "podcast_id=? AND title=?", podcast.ID, "Episode 126").Error)
	require.Len(t, pe, 2)

	require.NoError(t, podcasts.DownloadEpisode(pe[0].ID))
	require.NoError(t, podcasts.DownloadEpisode(pe[1].ID))

	require.NoError(t, m.DB().Order("id").Preload("Podcast").Find(&pe, "podcast_id=? AND title=?", podcast.ID, "Episode 126").Error)
	require.Len(t, pe, 2)

	// check we made a unique podcast episode names
	require.Equal(t, "InternetBoxEpisode126.mp3", pe[0].Filename)
	require.Equal(t, "InternetBoxEpisode126 (1).mp3", pe[1].Filename)
}

func TestGetMoreRecentEpisodes(t *testing.T) {
	t.Parallel()

	fp := gofeed.NewParser()
	newFeed, err := fp.Parse(bytes.NewReader(testRSS))
	if err != nil {
		t.Fatalf("parse test data: %v", err)
	}
	after, err := time.Parse(time.RFC1123, "Mon, 27 Jun 2016 06:33:43 +0000")
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	entries := getEntriesAfterDate(newFeed.Items, after)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}
