package transcode_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/cache"
	"go.senan.xyz/gonic/transcode"
)

var testProfile = transcode.PCM16le

const (
	// assuming above profile is 48kHz 16bit stereo
	sampleRate     = 48_000
	bytesPerSample = 2
	numChannels    = 2
)

const bytesPerSec = sampleRate * bytesPerSample * numChannels

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
}

// codecs are resolved by name, suffix and MIME, so each of those must identify exactly one codec
func TestCodecsUnique(t *testing.T) {
	t.Parallel()

	claimed := map[string]transcode.CodecName{}
	for name, codec := range transcode.Codecs {
		for _, alias := range []string{string(codec.Name), codec.Suffix, codec.MIME, strings.TrimPrefix(codec.MIME, "audio/")} {
			if prev, ok := claimed[alias]; ok {
				require.Equal(t, name, prev, "%q identifies two codecs", alias)
			}
			claimed[alias] = name
		}
	}
}

// TestTranscode starts a web server that transcodes a 5s FLAC file to PCM audio. A client
// consumes the result over a 5 second period.

func TestTranscode(t *testing.T) {
	t.Parallel()
	requireFFmpeg(t)

	testFile := "testdata/5s.flac"
	testFileLen := 5

	tr := transcode.NewFFmpegTranscoder()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, tr.Transcode(r.Context(), testProfile, testFile, w))
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	resp, err := server.Client().Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	var buf bytes.Buffer
	for {
		n, err := io.Copy(&buf, io.LimitReader(resp.Body, bytesPerSec))
		require.NoError(t, err)
		if n == 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// we should have 5 seconds of PCM data
	require.Equal(t, testFileLen*bytesPerSec, buf.Len())
}

// TestTranscodeWithSeek starts a web server that transcodes a 5s FLAC file to PCM audio, but with a 2 second offset.
// A client consumes the result over a 3 second period.
func TestTranscodeWithSeek(t *testing.T) {
	t.Parallel()
	requireFFmpeg(t)

	testFile := "testdata/5s.flac"
	testFileLen := 5

	seekSecs := 2
	profile := transcode.WithSeek(testProfile, time.Duration(seekSecs)*time.Second)

	tr := transcode.NewFFmpegTranscoder()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, tr.Transcode(r.Context(), profile, testFile, w))
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	resp, err := server.Client().Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	var buf bytes.Buffer
	for {
		n, err := io.Copy(&buf, io.LimitReader(resp.Body, bytesPerSec))
		require.NoError(t, err)
		if n == 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// since we seeked 2 seconds, we should have 5-2 = 3 seconds of PCM data
	require.Equal(t, (testFileLen-seekSecs)*bytesPerSec, buf.Len())
}

// TestTranscodeFLAC transcodes the 5s 48kHz FLAC down to 24kHz 16 bit, decodes the result back to PCM, and
// checks the durations match -- covering the flac profile's <sampleformat> expansion end to end.
func TestTranscodeFLAC(t *testing.T) {
	t.Parallel()
	requireFFmpeg(t)

	profile := transcode.WithBitDepth(transcode.WithSampleRate(transcode.FLAC, 24_000), 16)

	var buf bytes.Buffer
	tr := transcode.NewFFmpegTranscoder()
	require.NoError(t, tr.Transcode(context.Background(), profile, "testdata/5s.flac", &buf))

	f, err := os.CreateTemp(t.TempDir(), "*.flac")
	require.NoError(t, err)
	_, err = f.Write(buf.Bytes())
	require.NoError(t, err)
	require.NoError(t, f.Close())

	var pcm bytes.Buffer
	require.NoError(t, tr.Transcode(context.Background(), transcode.PCM16le, f.Name(), &pcm))
	require.Equal(t, 5*bytesPerSec, pcm.Len()) // PCM16le resamples to 48kHz, so 5s at the usual rate
}

func TestCachingParallelism(t *testing.T) {
	t.Parallel()
	requireFFmpeg(t)

	var realTranscodeCount atomic.Uint64
	transcoder := callbackTranscoder{
		transcoder: transcode.NewFFmpegTranscoder(),
		callback:   func() { realTranscodeCount.Add(1) },
	}

	cacheTranscoder := transcode.NewCachingTranscoder(transcoder, cache.New(t.TempDir(), 1024))

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			var buf bytes.Buffer
			require.NoError(t, cacheTranscoder.Transcode(context.Background(), transcode.PCM16le, "testdata/5s.flac", &buf))
			require.Equal(t, 5*bytesPerSec, buf.Len())
		})
	}

	wg.Wait()

	require.Equal(t, 1, int(realTranscodeCount.Load()))
}

type callbackTranscoder struct {
	transcoder transcode.Transcoder
	callback   func()
}

func (ct callbackTranscoder) Transcode(ctx context.Context, profile transcode.Profile, in string, out io.Writer) error {
	ct.callback()
	return ct.transcoder.Transcode(ctx, profile, in, out)
}
