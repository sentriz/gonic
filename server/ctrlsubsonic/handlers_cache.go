package ctrlsubsonic

import (
	"fmt"
	"net/http"
	"path"

	"io"
	"os"
	"os/exec"

	"github.com/cespare/xxhash"
)

type encoderProfile struct {
	format        string
	bitrate       int
	ffmpegOptions []string
	forceRG       bool
}

var (
	encProfiles = map[string]*encoderProfile{
		"mp3":     {"mp3", 128, []string{"-c:a", "libmp3lame"}, false},
		"mp3_rg":  {"mp3", 128, []string{"-c:a", "libmp3lame"}, true},
		"opus":    {"opus", 96, []string{"-c:a", "libopus", "-vbr", "constrained"}, false},
		"opus_rg": {"opus", 96, []string{"-c:a", "libopus", "-vbr", "constrained"}, true},
	}
	bufLen = 4096
)

func streamTrack(w http.ResponseWriter, r *http.Request, trackPath string, client string, clBitrate int, cachePath string) {
	// Guess required format based on client:
	profileName := detectFormat(client)
	profile := encProfiles[profileName]
	bitrate := getBitrate(clBitrate, profile)

	cacheFile := path.Join(cachePath, getCacheKey(trackPath, profileName, bitrate))

	if fileExists(cacheFile) {
		fmt.Printf("`%s`: cache [%s/%s] hit!\n", trackPath, profile.format, bitrate)
		http.ServeFile(w, r, cacheFile)
	} else {
		fmt.Printf("`%s`: cache [%s/%s] miss!\n", trackPath, profile.format, bitrate)
		encodeTrack(w, trackPath, cacheFile, profile, bitrate)
	}
}

func encodeTrack(w http.ResponseWriter, trackPath string, cachePath string, profile *encoderProfile, bitrate string) {
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
	go writeCmdOutput(w, cacheFile, pipeReader)

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

	fmt.Printf("`%s`: Encoded track to [%s/%s] successfully\n", trackPath, profile.format, bitrate)
}

// Copy command output to HTTP response body using io.Copy (simpler, but may increase TTFB)
func copyCmdOutput(res http.ResponseWriter, cache *os.File, pipeReader *io.PipeReader) {
	// Set up a MultiWriter to feed the command output
	// to both cache file and HTTP response:
	w := io.MultiWriter(res, cache)

	// Start copying!
	if _, err := io.Copy(w, pipeReader); err != nil {
		fmt.Printf("Error while writing encoded output: %s\n", err)
	}
}

// Copy command output to HTTP response manually with a buffer (should reduce TTFB)
func writeCmdOutput(res http.ResponseWriter, cache *os.File, pipeReader *io.PipeReader) {
	buffer := make([]byte, bufLen)
	for {
		n, err := pipeReader.Read(buffer)
		if err != nil {
			pipeReader.Close()
			break
		}

		data := buffer[0:n]
		_, err = res.Write(data)
		if err != nil {
			fmt.Printf("Error while writing HTTP response: %s\n", err)
		}

		_, err = cache.Write(data)
		if err != nil {
			fmt.Printf("Error while writing cache file: %s\n", err)
		}

		if f, ok := res.(http.Flusher); ok {
			f.Flush()
		}
		//reset buffer
		for i := 0; i < n; i++ {
			buffer[i] = 0
		}
	}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Pre-format the FFmpeg command with needed options:
func ffmpegCommand(filePath string, profile *encoderProfile, bitrate string) *exec.Cmd {
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
	ffmpegArgs = append(ffmpegArgs, "-f", profile.format, "-")

	return exec.Command("/usr/bin/ffmpeg", ffmpegArgs...)
}

// Put special clients that can't handle Opus here:
func detectFormat(client string) (profile string) {
	if client == "Soundwaves" {
		return "mp3_rg"
	}
	if client == "Jamstash" {
		return "opus_rg"
	}
	return "opus"
}

// Generate cache key (file name). For, you know, encoded tracks cache.
func getCacheKey(sourcePath string, profile string, bitrate string) string {
	format := encProfiles[profile].format
	return fmt.Sprintf("%x-%s-%s.%s", xxhash.Sum64String(sourcePath), profile, bitrate, format)
}

// Check if client forces bitrate lower than set in profile:
func getBitrate(clientBitrate int, profile *encoderProfile) string {
	bitrate := profile.bitrate
	if clientBitrate != 0 && clientBitrate < profile.bitrate {
		bitrate = clientBitrate
	}
	return fmt.Sprintf("%dk", bitrate)
}
