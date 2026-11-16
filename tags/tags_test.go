package tags

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.senan.xyz/wrtag/tags/normtag"
)

func TestReadCreditsJoinedNameMultipleMBIDs(t *testing.T) {
	t.Parallel()

	const (
		mbidA = "11111111-1111-1111-1111-111111111111"
		mbidB = "22222222-2222-2222-2222-222222222222"
	)

	trags := Tags{}
	normtag.Set(trags, normtag.AlbumArtist, "Aesop Rock × Blockhead")
	normtag.Set(trags, normtag.AlbumArtists, "Aesop Rock", "Blockhead")
	normtag.Set(trags, normtag.MusicBrainzAlbumArtistID, mbidA, mbidB)

	// default (none) mode reads the single joined name, it must not adopt the first artist's MBID
	none := map[*Spec]MultiValueSetting{AlbumArtist: {Mode: None}}
	assert.Equal(t, []Credited{{Value: "Aesop Rock × Blockhead"}}, ReadCredits(trags, AlbumArtist, none))

	// multi mode still pairs each name with its MBID
	multi := map[*Spec]MultiValueSetting{AlbumArtist: {Mode: Multi}}
	assert.Equal(t, []Credited{
		{Value: "Aesop Rock", MusicBrainzID: mbidA},
		{Value: "Blockhead", MusicBrainzID: mbidB},
	}, ReadCredits(trags, AlbumArtist, multi))

	// a single artist with a single MBID still pairs in none mode
	solo := Tags{}
	normtag.Set(solo, normtag.AlbumArtist, "Aesop Rock")
	normtag.Set(solo, normtag.MusicBrainzAlbumArtistID, mbidA)
	assert.Equal(t, []Credited{{Value: "Aesop Rock", MusicBrainzID: mbidA}}, ReadCredits(solo, AlbumArtist, none))
}
