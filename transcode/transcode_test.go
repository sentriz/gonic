package transcode_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return // no ffmpeg, skip these tests
	}
	os.Exit(m.Run())
}

// TestTranscode starts a web server that transcodes a 5s FLAC file to PCM audio. A client
// consumes the result over a 5 second period.
func TestTranscode(t *testing.T) {
	t.Parallel()

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
