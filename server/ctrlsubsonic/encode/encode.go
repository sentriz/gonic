package encode

import (
	"fmt"
	"net/http"

	"io"
	"os"
	"os/exec"

	"github.com/cespare/xxhash"
)

type Profile struct {
	Format        string
	Bitrate       string
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

// Copy command output to HTTP response body using io.Copy (simpler, but may increase TTFB)
//nolint:deadcode,unused
func copyCmdOutput(out, cache io.Writer, pipeReader io.Reader) {
	// Set up a MultiWriter to feed the command output
	// to both cache file and HTTP response:
	w := io.MultiWriter(out, cache)

	// Start copying!
	if _, err := io.Copy(w, pipeReader); err != nil {
		fmt.Printf("Error while writing encoded output: %s\n", err)
	}
}

// Copy command output to HTTP response manually with a buffer (should reduce TTFB)
//nolint:deadcode,unused
func writeCmdOutput(out, cache io.Writer, pipeReader io.ReadCloser) {
	buffer := make([]byte, bufLen)
	for {
		n, err := pipeReader.Read(buffer)
		if err != nil {
			pipeReader.Close()
			break
		}

		data := buffer[0:n]
		_, err = out.Write(data)
		if err != nil {
			fmt.Printf("Error while writing HTTP response: %s\n", err)
		}

		_, err = cache.Write(data)
		if err != nil {
			fmt.Printf("Error while writing cache file: %s\n", err)
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

// Pre-format the FFmpeg command with needed options:
func ffmpegCommand(filePath string, profile *Profile, bitrate string) *exec.Cmd {
	ffmpegArgs := []string{
		"-v", "0", "-i", filePath, "-map", "0:0",
		"-vn", "-b:a", bitrate,
	}
	ffmpegArgs = append(ffmpegArgs, profile.ffmpegOptions...)
	if profile.forceRG {
		ffmpegArgs = append(ffmpegArgs,
			// Set up ReplayGain processing
			"-af", "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled",
			// Drop redundant ReplayGain tags
			"-metadata", "replaygain_album_gain=",
			"-metadata", "replaygain_album_peak=",
			"-metadata", "replaygain_track_gain=",
			"-metadata", "replaygain_track_peak=",
		)
	}
	ffmpegArgs = append(ffmpegArgs, "-f", profile.Format, "-")

	return exec.Command("/usr/bin/ffmpeg", ffmpegArgs...)
}

func Encode(out io.Writer, trackPath, cachePath string, profile *Profile, bitrate string) error {
	// Prepare the command and file descriptors:
	cmd := ffmpegCommand(trackPath, profile, bitrate)
	pipeReader, pipeWriter := io.Pipe()
	cmd.Stdout = pipeWriter
	cmd.Stderr = pipeWriter

	// Create cache file:
	cacheFile, err := os.Create(cachePath)
	if err != nil {
		fmt.Printf("Failed to write to cache file `%s`: %s\n", cachePath, err)
	}

	//// I'm still unsure if buffer version (writeCmdOutput) is any better than io.Copy-based one (copyCmdOutput).
	//// My initial goal here is to start streaming response ASAP, with smallest TTFB. More testing needed. -- @spijet
	// Start up writers for cache file and HTTP response:
	// go copyCmdOutput(w, cacheFile, pipeReader)
	go writeCmdOutput(out, cacheFile, pipeReader)

	// Run FFmpeg:
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Failed to encode `%s`: %s\n", trackPath, err)
	}

	// Close all pipes and flush cache file:
	pipeWriter.Close()
	err = cacheFile.Sync()
	if err != nil {
		fmt.Printf("Failed to flush `%s`: %s\n", cachePath, err)
	}
	cacheFile.Close()

	fmt.Printf("`%s`: Encoded track to [%s/%s] successfully\n",
		trackPath, profile.Format, profile.Bitrate)
}

// Generate cache key (file name). For, you know, encoded tracks cache.
func CacheKey(sourcePath string, profile string, bitrate string) string {
	format := Profiles[profile].Format
	return fmt.Sprintf("%x-%s-%s.%s", xxhash.Sum64String(sourcePath), profile, bitrate, format)
}

// Check if client forces bitrate lower than set in profile:
func GetBitrate(clientBitrate int, profile *Profile) string {
	bitrate := profile.Bitrate
	if clientBitrate != 0 && clientBitrate < profile.Bitrate {
		bitrate = clientBitrate
	}
	return fmt.Sprintf("%dk", bitrate)
}
