package ctrlsubsonic

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/cache"
)

func TestGetCoverArtSourceChanged(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.contr.coverCache = cache.New(t.TempDir(), 10)

	coverPath := filepath.Join(f.m.TmpDir(), "m-0", "artist-a", "album-ab", "cover.png")
	writeCover := func(c color.Color, encode func(io.Writer, image.Image) error, modTime time.Time) {
		img := image.NewRGBA(image.Rect(0, 0, 4, 4))
		for x := range 4 {
			for y := range 4 {
				img.Set(x, y, c)
			}
		}
		file, err := os.Create(coverPath)
		require.NoError(t, err)
		require.NoError(t, encode(file, img))
		require.NoError(t, file.Close())
		require.NoError(t, os.Chtimes(coverPath, modTime, modTime))
	}
	getCover := func(size int) color.RGBA {
		w, r := makeHTTPMock(url.Values{"id": {f.albumAB.SID().String()}, "size": {fmt.Sprint(size)}}, f.admin)
		require.Nil(t, f.contr.ServeGetCoverArt(w, r))
		img, _, err := image.Decode(w.Body)
		require.NoError(t, err)
		return color.RGBAModel.Convert(img.At(0, 0)).(color.RGBA)
	}

	writeCover(color.RGBA{R: 255, A: 255}, png.Encode, time.Now().Add(-2*time.Hour))
	for _, size := range []int{2, 600} {
		c := getCover(size)
		require.Greater(t, c.R, c.B)
	}

	entries, err := os.ReadDir(f.contr.coverCache.Path())
	require.NoError(t, err)
	for _, entry := range entries {
		cachedAt := time.Now().Add(-time.Hour)
		require.NoError(t, os.Chtimes(filepath.Join(f.contr.coverCache.Path(), entry.Name()), cachedAt, cachedAt))
	}

	jpegEncode := func(w io.Writer, img image.Image) error { return jpeg.Encode(w, img, nil) }
	writeCover(color.RGBA{B: 255, A: 255}, jpegEncode, time.Now().Add(-3*time.Hour))
	for _, size := range []int{2, 600} {
		c := getCover(size)
		require.Greater(t, c.B, c.R)
	}
}
