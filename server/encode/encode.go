// author: spijet (https://github.com/spijet/)

package encode

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/cespare/xxhash"
)

type Profile struct {
	Format        string
	Bitrate       int
	ffmpegOptions []string
	forceRG       bool
}

var (
	Profiles = map[string]*Profile{
		"mp3":     {"mp3", 128, []string{"-c:a", "libmp3lame"}, false},
		"mp3_rg":  {"mp3", 128, []string{"-c:a", "libmp3lame"}, true},
		"opus":    {"opus", 96, []string{"-c:a", "libopus", "-vbr", "constrained"}, false},
		"opus_rg": {"opus", 96, []string{"-c:a", "libopus", "-vbr", "constrained"}, true},
	}
	bufLen = 4096
)

// copy command output to http response body using io.copy (simpler, but may increase ttfb)
//nolint:deadcode,unused
func copyCmdOutput(out, cache io.Writer, pipeReader io.Reader) {
	// set up a multiwriter to feed the command output
	// to both cache file and http response
	w := io.MultiWriter(out, cache)
	// start copying!
	if _, err := io.Copy(w, pipeReader); err != nil {
		log.Printf("error while writing encoded output: %s\n", err)
	}
}

// copy command output to http response manually with a buffer (should reduce ttfb)
//nolint:deadcode,unused
func writeCmdOutput(out, cache io.Writer, pipeReader io.ReadCloser) {
	buffer := make([]byte, bufLen)
	for {
		n, err := pipeReader.Read(buffer)
		if err != nil {
			_ = pipeReader.Close()
			break
		}
		data := buffer[0:n]
		if _, err := out.Write(data); err != nil {
			log.Printf("error while writing HTTP response: %s\n", err)
		}
		if _, err := cache.Write(data); err != nil {
			log.Printf("error while writing cache file: %s\n", err)
		}
		if f, ok := out.(http.Flusher); ok {
			f.Flush()
		}
		// reset buffer
		for i := 0; i < n; i++ {
			buffer[i] = 0
		}
	}
}

// pre-format the ffmpeg command with needed options
func ffmpegCommand(filePath string, profile *Profile, bitrate string) *exec.Cmd {
	args := []string{
		"-v", "0",
		"-i", filePath,
		"-map", "0:0",
		"-vn",
		"-b:a", bitrate,
	}
	args = append(args, profile.ffmpegOptions...)
	if profile.forceRG {
		args = append(args,
			// set up replaygain processing
			"-af", "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled",
			// drop redundant replaygain tags
			"-metadata", "replaygain_album_gain=",
			"-metadata", "replaygain_album_peak=",
			"-metadata", "replaygain_track_gain=",
			"-metadata", "replaygain_track_peak=",
		)
	}
	args = append(args, "-f", profile.Format, "-")
	return exec.Command("/usr/bin/ffmpeg", args...) //nolint:gosec
}

func Encode(out io.Writer, trackPath, cachePath string, profile *Profile, bitrate string) error {
	// prepare the command and file descriptors
	cmd := ffmpegCommand(trackPath, profile, bitrate)
	pipeReader, pipeWriter := io.Pipe()
	cmd.Stdout = pipeWriter
	cmd.Stderr = pipeWriter
	// create cache file
	cacheFile, err := os.Create(cachePath)
	if err != nil {
		return fmt.Errorf("writing to cache file %q: %v: %w", cachePath, err, err)
	}
	// still unsure if buffer version (writeCmdOutput) is any better than io.Copy-based one (copyCmdOutput)
	// initial goal here is to start streaming response asap, with smallest ttfb. more testing needed
	// -- @spijet
	//
	// start up writers for cache file and http response
	go writeCmdOutput(out, cacheFile, pipeReader)
	// run ffmpeg
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running ffmpeg: %w", err)
	}
	// close all pipes and flush cache file
	_ = pipeWriter.Close()
	if err := cacheFile.Sync(); err != nil {
		return fmt.Errorf("flushing %q: %w", cachePath, err)
	}
	_ = cacheFile.Close()
	return nil
}

// CacheKey generates the filename for the new transcode save
func CacheKey(sourcePath string, profile, bitrate string) string {
	format := Profiles[profile].Format
	hash := xxhash.Sum64String(sourcePath)
	return fmt.Sprintf("%x-%s-%s.%s", hash, profile, bitrate, format)
}

// GetBitrate checks if the client forces bitrate lower than set in profile
func GetBitrate(clientBitrate int, profile *Profile) string {
	bitrate := profile.Bitrate
	if clientBitrate != 0 && clientBitrate < bitrate {
		bitrate = clientBitrate
	}
	return fmt.Sprintf("%dk", bitrate)
}
