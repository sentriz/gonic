package transcode_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/transcode"
)

// TestTranscode starts a web server that transcodes a 5s FLAC file to PCM audio. A client
// consumes the result over a 5 second period.
func TestTranscode(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH")
	}

	tr := transcode.NewFFmpegTranscoder()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, tr.Transcode(r.Context(), transcode.PCM16le, "testdata/5s.flac", w))
		f, ok := w.(http.Flusher)
		require.True(t, ok)
		f.Flush()
	}))
	defer server.Close()

	resp, err := server.Client().Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	const sampleRate, bytesPerSample, numChannels = 48_000, 2, 2
	const bytesPerSec = sampleRate * bytesPerSample * numChannels

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
	require.Equal(t, 5*bytesPerSec, buf.Len())
}
