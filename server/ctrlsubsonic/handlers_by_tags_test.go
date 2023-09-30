package ctrlsubsonic

import (
	"net/url"
	"testing"
)

func TestGetArtists(t *testing.T) {
	t.Parallel()
	contr := makeControllerRoots(t, []string{"m-0", "m-1"})
	runQueryCases(t, contr.ServeGetArtists, []*queryCase{
		{url.Values{}, "no_args", false},
		{url.Values{"musicFolderId": {"0"}}, "with_music_folder_1", false},
		{url.Values{"musicFolderId": {"1"}}, "with_music_folder_2", false},
	})
}

func TestGetArtist(t *testing.T) {
	t.Parallel()
	contr := makeController(t)
	runQueryCases(t, contr.ServeGetArtist, []*queryCase{
		{url.Values{"id": {"ar-1"}}, "id_one", false},
		{url.Values{"id": {"ar-2"}}, "id_two", false},
		{url.Values{"id": {"ar-3"}}, "id_three", false},
	})
}

func TestGetAlbum(t *testing.T) {
	t.Parallel()
	contr := makeController(t)
	runQueryCases(t, contr.ServeGetAlbum, []*queryCase{
		{url.Values{"id": {"al-2"}}, "without_cover", false},
		{url.Values{"id": {"al-3"}}, "with_cover", false},
	})
}

func TestGetAlbumListTwo(t *testing.T) {
	t.Parallel()
	contr := makeController(t)
	runQueryCases(t, contr.ServeGetAlbumListTwo, []*queryCase{
		{url.Values{"type": {"alphabeticalByArtist"}}, "alpha_artist", false},
		{url.Values{"type": {"alphabeticalByName"}}, "alpha_name", false},
		{url.Values{"type": {"newest"}}, "newest", false},
		{url.Values{"type": {"random"}, "size": {"15"}}, "random", true},
	})
}

func TestSearchThree(t *testing.T) {
	t.Parallel()
	contr := makeController(t)
	runQueryCases(t, contr.ServeSearchThree, []*queryCase{
		{url.Values{"query": {"art"}}, "q_art", false},
		{url.Values{"query": {"alb"}}, "q_alb", false},
		{url.Values{"query": {"tit"}}, "q_tra", false},
	})
}
