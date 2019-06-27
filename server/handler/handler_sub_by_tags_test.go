package handler

import (
	"net/url"
	"testing"
)

func TestGetArtists(t *testing.T) {
	testQueryCases(t, testController.GetArtists, []*queryCase{
		{url.Values{}, "no_args", false},
	})
}

func TestGetArtist(t *testing.T) {
	testQueryCases(t, testController.GetArtist, []*queryCase{
		{url.Values{"id": []string{"1"}}, "id_one", false},
		{url.Values{"id": []string{"2"}}, "id_two", false},
		{url.Values{"id": []string{"3"}}, "id_three", false},
	})
}

func TestGetAlbum(t *testing.T) {
	testQueryCases(t, testController.GetAlbum, []*queryCase{
		{url.Values{"id": []string{"2"}}, "without_cover", false},
		{url.Values{"id": []string{"3"}}, "with_cover", false},
	})
}

func TestGetAlbumListTwo(t *testing.T) {
	testQueryCases(t, testController.GetAlbumListTwo, []*queryCase{
		{url.Values{"type": []string{"alphabeticalByArtist"}}, "alpha_artist", false},
		{url.Values{"type": []string{"alphabeticalByName"}}, "alpha_name", false},
		{url.Values{"type": []string{"newest"}}, "newest", false},
		{url.Values{"type": []string{"random"}, "size": []string{"15"}}, "random", true},
	})
}

func TestSearchThree(t *testing.T) {
	testQueryCases(t, testController.SearchThree, []*queryCase{
		{url.Values{"query": []string{"13"}}, "q_13", false},
		{url.Values{"query": []string{"ani"}}, "q_ani", false},
		{url.Values{"query": []string{"cert"}}, "q_cert", false},
	})
}
