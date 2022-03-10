package ctrlsubsonic

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/matryer/is"
	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/server/transcode"
)

func TestServeStreamRaw(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("no ffmpeg in $PATH")
	}

	is := is.New(t)
	contr := makeControllerAudio(t)

	statFlac := stat(t, audioPath10s)

	rr, req := makeHTTPMock(url.Values{"id": {"tr-1"}})
	serveRaw(t, contr, contr.ServeStream, rr, req)

	is.Equal(rr.Code, http.StatusOK)
	is.Equal(rr.Header().Get("content-type"), "audio/flac")
	is.Equal(atoi(t, rr.Header().Get("content-length")), int(statFlac.Size()))
	is.Equal(atoi(t, rr.Header().Get("content-length")), rr.Body.Len())
}

func TestServeStreamOpus(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("no ffmpeg in $PATH")
	}

	is := is.New(t)
	contr := makeControllerAudio(t)

	var user db.User
	is.NoErr(contr.DB.Where("name=?", mockUsername).Find(&user).Error)
	is.NoErr(contr.DB.Create(&db.TranscodePreference{UserID: user.ID, Client: mockClientName, Profile: "opus"}).Error)

	rr, req := makeHTTPMock(url.Values{"id": {"tr-1"}})
	serveRaw(t, contr, contr.ServeStream, rr, req)

	is.Equal(rr.Code, http.StatusOK)
	is.Equal(rr.Header().Get("content-type"), "audio/ogg")
	is.Equal(atoi(t, rr.Header().Get("content-length")), transcode.GuessExpectedSize(transcode.Opus, 10*time.Second))
	is.Equal(atoi(t, rr.Header().Get("content-length")), rr.Body.Len())
}

func TestServeStreamOpusMaxBitrate(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("no ffmpeg in $PATH")
	}

	is := is.New(t)
	contr := makeControllerAudio(t)

	var user db.User
	is.NoErr(contr.DB.Where("name=?", mockUsername).Find(&user).Error)
	is.NoErr(contr.DB.Create(&db.TranscodePreference{UserID: user.ID, Client: mockClientName, Profile: "opus"}).Error)

	const bitrate = 5

	rr, req := makeHTTPMock(url.Values{"id": {"tr-1"}, "maxBitRate": {strconv.Itoa(bitrate)}})
	serveRaw(t, contr, contr.ServeStream, rr, req)

	profile := transcode.WithBitrate(transcode.Opus, transcode.BitRate(bitrate))
	expectedLength := transcode.GuessExpectedSize(profile, 10*time.Second)

	is.Equal(rr.Code, http.StatusOK)
	is.Equal(rr.Header().Get("content-type"), "audio/ogg")
	is.Equal(atoi(t, rr.Header().Get("content-length")), expectedLength)
	is.Equal(atoi(t, rr.Header().Get("content-length")), rr.Body.Len())
}

func TestServeStreamMP3Range(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("no ffmpeg in $PATH")
	}

	is := is.New(t)
	contr := makeControllerAudio(t)

	var user db.User
	is.NoErr(contr.DB.Where("name=?", mockUsername).Find(&user).Error)
	is.NoErr(contr.DB.Create(&db.TranscodePreference{UserID: user.ID, Client: mockClientName, Profile: "mp3"}).Error)

	var totalBytes []byte
	{
		rr, req := makeHTTPMock(url.Values{"id": {"tr-1"}})
		serveRaw(t, contr, contr.ServeStream, rr, req)
		is.Equal(rr.Code, http.StatusOK)
		is.Equal(rr.Header().Get("content-type"), "audio/mpeg")
		totalBytes = rr.Body.Bytes()
	}

	const chunkSize = 2 << 16

	var bytes []byte
	for i := 0; i < len(totalBytes); i += chunkSize {
		rr, req := makeHTTPMock(url.Values{"id": {"tr-1"}})
		req.Header.Set("range", fmt.Sprintf("bytes=%d-%d", i, min(i+chunkSize, len(totalBytes))-1))
		t.Log(req.Header.Get("range"))
		serveRaw(t, contr, contr.ServeStream, rr, req)
		is.Equal(rr.Code, http.StatusPartialContent)
		is.Equal(rr.Header().Get("content-type"), "audio/mpeg")
		is.True(atoi(t, rr.Header().Get("content-length")) == chunkSize || atoi(t, rr.Header().Get("content-length")) == len(totalBytes)%chunkSize)
		is.Equal(atoi(t, rr.Header().Get("content-length")), rr.Body.Len())
		bytes = append(bytes, rr.Body.Bytes()...)
	}

	is.Equal(len(totalBytes), len(bytes))
	is.Equal(totalBytes, bytes)
}

func stat(t *testing.T, path string) fs.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	return info
}

func atoi(t *testing.T, in string) int {
	t.Helper()
	i, err := strconv.Atoi(in)
	if err != nil {
		t.Fatalf("atoi %q: %v", in, err)
	}
	return i
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
