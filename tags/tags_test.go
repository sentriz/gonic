package tags

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.senan.xyz/wrtag/tags/normtag"
)

func TestReadCredits(t *testing.T) {
	t.Parallel()

	none := MultiValueSetting{Mode: None}
	multi := MultiValueSetting{Mode: Multi}
	delim := func(d string) MultiValueSetting { return MultiValueSetting{Mode: Delim, Delim: d} }

	tests := []struct {
		name    string
		spec    *Spec
		setting MultiValueSetting
		tags    Tags
		want    []Credited
	}{
		{
			"none joined name has no mbid", Artist, none,
			Tags{normtag.Artist: {"A × B"}, normtag.ArtistCredit: {"A with B"}, normtag.Artists: {"A", "B"}, normtag.MusicBrainzArtistID: {"id-a", "id-b"}},
			[]Credited{{Value: "A × B", ValueCredit: "A with B"}},
		},
		{
			"delim unsplit joined name has no mbid", Artist, delim(";"),
			Tags{normtag.Artist: {"A × B"}, normtag.ArtistCredit: {"A with B"}, normtag.Artists: {"A", "B"}, normtag.MusicBrainzArtistID: {"id-a", "id-b"}},
			[]Credited{{Value: "A × B", ValueCredit: "A with B"}},
		},
		{
			"delim split pairs mbids", Artist, delim("×"),
			Tags{normtag.Artist: {"A × B"}, normtag.ArtistCredit: {"A with B"}, normtag.Artists: {"A", "B"}, normtag.MusicBrainzArtistID: {"id-a", "id-b"}},
			[]Credited{{Value: "A", MusicBrainzID: "id-a"}, {Value: "B", MusicBrainzID: "id-b"}},
		},
		{
			"multi pairs mbids", Artist, multi,
			Tags{normtag.Artist: {"A × B"}, normtag.ArtistCredit: {"A with B"}, normtag.Artists: {"A", "B"}, normtag.MusicBrainzArtistID: {"id-a", "id-b"}},
			[]Credited{{Value: "A", MusicBrainzID: "id-a"}, {Value: "B", MusicBrainzID: "id-b"}},
		},
		{
			"none solo", Artist, none,
			Tags{normtag.Artist: {"A"}, normtag.ArtistCredit: {"A credited"}, normtag.MusicBrainzArtistID: {"id-a"}},
			[]Credited{{Value: "A", ValueCredit: "A credited", MusicBrainzID: "id-a"}},
		},
		{
			"none multi field artist pairs mbids", Artist, none,
			Tags{normtag.Artist: {"A", "B"}, normtag.MusicBrainzArtistID: {"id-a", "id-b"}},
			[]Credited{{Value: "A", MusicBrainzID: "id-a"}, {Value: "B", MusicBrainzID: "id-b"}},
		},
		{
			"multi pairs credits", Artist, multi,
			Tags{normtag.Artists: {"A", "B"}, normtag.ArtistsCredit: {"A credited", "B credited"}},
			[]Credited{{Value: "A", ValueCredit: "A credited"}, {Value: "B", ValueCredit: "B credited"}},
		},
		{
			"multi empty credit uses name", Artist, multi,
			Tags{normtag.Artists: {"A", "B"}, normtag.ArtistsCredit: {"A credited", ""}},
			[]Credited{{Value: "A", ValueCredit: "A credited"}, {Value: "B"}},
		},
		{
			"multi single display credit not paired", Artist, multi,
			Tags{normtag.Artists: {"A", "B"}, normtag.ArtistCredit: {"A with B"}},
			[]Credited{{Value: "A"}, {Value: "B"}},
		},
		{
			"multi mbid count mismatch not paired", Artist, multi,
			Tags{normtag.Artists: {"A", "B", "C"}, normtag.MusicBrainzArtistID: {"id-a", "id-b"}},
			[]Credited{{Value: "A"}, {Value: "B"}, {Value: "C"}},
		},
		{
			"delim splits joined mbids", Artist, delim(";"),
			Tags{normtag.Artist: {"A; B"}, normtag.MusicBrainzArtistID: {"id-a; id-b"}},
			[]Credited{{Value: "A", MusicBrainzID: "id-a"}, {Value: "B", MusicBrainzID: "id-b"}},
		},
		{
			"none no tags falls back", Artist, none,
			Tags{},
			[]Credited{{Value: "Unknown Artist"}},
		},
		{
			"multi no tags falls back", Artist, multi,
			Tags{},
			[]Credited{{Value: "Unknown Artist"}},
		},
		{
			"album artist does not take the track artist mbid", AlbumArtist, none,
			Tags{normtag.AlbumArtist: {"Various Artists"}, normtag.Artist: {"A"}, normtag.MusicBrainzArtistID: {"id-a"}},
			[]Credited{{Value: "Various Artists"}},
		},
		{
			"album artist falls back to track artist with mbid", AlbumArtist, none,
			Tags{normtag.Artist: {"A"}, normtag.MusicBrainzArtistID: {"id-a"}},
			[]Credited{{Value: "A", MusicBrainzID: "id-a"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ReadCredits(tt.tags, tt.spec, tt.setting))
		})
	}
}

func TestRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		spec             *Spec
		tags             Tags
		want, wantCredit string
	}{
		{
			"year takes first valid value", Year,
			Tags{normtag.OriginalDate: {"garbage", "2020"}, normtag.Date: {"2019"}},
			"2020", "",
		},
		{
			"year falls back to next source", Year,
			Tags{normtag.OriginalDate: {"garbage"}, normtag.Date: {"2019"}},
			"2019", "",
		},
		{
			"display credit kept for multi field artist", Artist,
			Tags{normtag.Artist: {"A", "B"}, normtag.ArtistCredit: {"A with B"}},
			"A", "A with B",
		},
		{
			"credit equal to value dropped", Artist,
			Tags{normtag.Artist: {"A"}, normtag.ArtistCredit: {"A"}},
			"A", "",
		},
		{
			"album artist falls back to artist with its credit", AlbumArtist,
			Tags{normtag.Artist: {"A"}, normtag.ArtistCredit: {"A credited"}},
			"A", "A credited",
		},
		{
			"multi source ignored", Artist,
			Tags{normtag.Artists: {"A", "B"}},
			"Unknown Artist", "",
		},
		{
			"no tags falls back", AlbumTitle,
			Tags{},
			"Unknown Album", "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			value, valueCredit := Read(tt.tags, tt.spec)
			assert.Equal(t, tt.want, value)
			assert.Equal(t, tt.wantCredit, valueCredit)
		})
	}
}
