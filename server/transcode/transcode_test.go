//go:build go1.18
// +build go1.18

package transcode_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/matryer/is"
	"go.senan.xyz/gonic/server/transcode"
)

// FuzzGuessExpectedSize makes sure all of our profile's estimated transcode
// file sizes are slightly bigger than the real thing.
func FuzzGuessExpectedSize(f *testing.F) {
	var profiles []transcode.Profile
	for _, v := range transcode.UserProfiles {
		profiles = append(profiles, v)
	}

	type track struct {
		path   string
		length time.Duration
	}
	var tracks []track
	tracks = append(tracks, track{"testdata/5s.mp3", 5 * time.Second})
	tracks = append(tracks, track{"testdata/10s.mp3", 10 * time.Second})

	tr := transcode.NewFFmpegTranscoder()
	f.Fuzz(func(t *testing.T, pseed uint8, tseed uint8) {
		is := is.New(t)
		profile := profiles[int(pseed)%len(profiles)]
		track := tracks[int(tseed)%len(tracks)]

		sizeGuess := transcode.GuessExpectedSize(profile, track.length)

		reader, err := tr.Transcode(context.Background(), profile, track.path)
		is.NoErr(err)

		actual, err := io.ReadAll(reader)
		is.NoErr(err)
		is.True(sizeGuess > len(actual))
	})
}
