package ctrlsubsonic

import (
	"net/url"
	"testing"
)

func TestGetArtists(t *testing.T) {
	runQueryCases(t, testController.ServeGetArtists, []*queryCase{
		{url.Values{}, "no_args", false},
	})
}

func TestGetArtist(t *testing.T) {
	runQueryCases(t, testController.ServeGetArtist, []*queryCase{
		{url.Values{"id": {"ar-1"}}, "id_one", false},
		{url.Values{"id": {"ar-2"}}, "id_two", false},
		{url.Values{"id": {"ar-3"}}, "id_three", false},
	})
}

func TestGetAlbum(t *testing.T) {
	runQueryCases(t, testController.ServeGetAlbum, []*queryCase{
		{url.Values{"id": {"al-2"}}, "without_cover", false},
		{url.Values{"id": {"al-3"}}, "with_cover", false},
	})
}

func TestGetAlbumListTwo(t *testing.T) {
	runQueryCases(t, testController.ServeGetAlbumListTwo, []*queryCase{
		{url.Values{"type": {"alphabeticalByArtist"}}, "alpha_artist", false},
		{url.Values{"type": {"alphabeticalByName"}}, "alpha_name", false},
		{url.Values{"type": {"newest"}}, "newest", false},
		{url.Values{"type": {"random"}, "size": {"15"}}, "random", true},
	})
}

func TestSearchThree(t *testing.T) {
	runQueryCases(t, testController.ServeSearchThree, []*queryCase{
		{url.Values{"query": {"13"}}, "q_13", false},
		{url.Values{"query": {"ani"}}, "q_ani", false},
		{url.Values{"query": {"cert"}}, "q_cert", false},
	})
}
