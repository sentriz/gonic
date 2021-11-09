package ctrlsubsonic

import (
	"net/url"
	"testing"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func TestGetIndexes(t *testing.T) {
	contr, m := makeControllerRoots(t, []string{"m-0", "m-1"})
	defer m.CleanUp()

	runQueryCases(t, contr, contr.ServeGetIndexes, []*queryCase{
		{url.Values{}, "no_args", false},
		{url.Values{"musicFolderId": {"0"}}, "with_music_folder_1", false},
		{url.Values{"musicFolderId": {"1"}}, "with_music_folder_2", false},
	})
}

func TestGetMusicDirectory(t *testing.T) {
	contr, m := makeController(t)
	defer m.CleanUp()

	runQueryCases(t, contr, contr.ServeGetMusicDirectory, []*queryCase{
		{url.Values{"id": {"al-2"}}, "without_tracks", false},
		{url.Values{"id": {"al-3"}}, "with_tracks", false},
	})
}

func TestGetAlbumList(t *testing.T) {
	t.Parallel()
	contr, m := makeController(t)
	defer m.CleanUp()

	runQueryCases(t, contr, contr.ServeGetAlbumList, []*queryCase{
		{url.Values{"type": {"alphabeticalByArtist"}}, "alpha_artist", false},
		{url.Values{"type": {"alphabeticalByName"}}, "alpha_name", false},
		{url.Values{"type": {"newest"}}, "newest", false},
		{url.Values{"type": {"random"}, "size": {"15"}}, "random", true},
	})
}

func TestSearchTwo(t *testing.T) {
	t.Parallel()
	contr, m := makeController(t)
	defer m.CleanUp()

	runQueryCases(t, contr, contr.ServeSearchTwo, []*queryCase{
		{url.Values{"query": {"art"}}, "q_art", false},
		{url.Values{"query": {"alb"}}, "q_alb", false},
		{url.Values{"query": {"tra"}}, "q_tra", false},
	})
}
