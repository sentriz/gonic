// author: spijet (https://github.com/spijet/)

package encode

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"

	"github.com/cespare/xxhash"
)

const (
	buffLen = 4096
	ffmpeg  = "ffmpeg"
)

type Profile struct {
	Format        string
	Bitrate       int
	ffmpegOptions []string
	forceRG       bool
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func Profiles() map[string]Profile {
	return map[string]Profile{
		"mp3":     {"mp3", 128, []string{"-c:a", "libmp3lame"}, false},
		"mp3_rg":  {"mp3", 128, []string{"-c:a", "libmp3lame"}, true},
		"opus":    {"opus", 96, []string{"-c:a", "libopus", "-vbr", "constrained"}, false},
		"opus_rg": {"opus", 96, []string{"-c:a", "libopus", "-vbr", "constrained"}, true},
	}
}

// copy command output to http response body using io.copy
// (it's simpler, but may increase ttfb)
//nolint:deadcode,unused // function may be switched later
func cmdOutputCopy(out, cache io.Writer, pipeReader io.Reader) {
	// set up a multiwriter to feed the command output
	// to both cache file and http response
	w := io.MultiWriter(out, cache)
	// start copying!
	if _, err := io.Copy(w, pipeReader); err != nil {
		log.Printf("error while writing encoded output: %s\n", err)
	}
}

// copy command output to http response manually with a buffer (should reduce ttfb)
//nolint:deadcode,unused // function may be switched later
func cmdOutputWrite(out, cache io.Writer, pipeReader io.ReadCloser) {
	buffer := make([]byte, buffLen)
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
func ffmpegCommand(filePath string, profile Profile) (*exec.Cmd, error) {
	args := []string{
		"-v", "0",
		"-i", filePath,
		"-map", "0:a:0",
		"-vn",
		"-b:a", fmt.Sprintf("%dk", profile.Bitrate),
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
			"-metadata", "r128_album_gain=",
			"-metadata", "r128_track_gain=",
		)
	}
	args = append(args, "-f", profile.Format, "-")
	ffmpegPath, err := exec.LookPath(ffmpeg)
	if err != nil {
		return nil, fmt.Errorf("find ffmpeg binary path: %w", err)
	}
	return exec.Command(ffmpegPath, args...), nil //nolint:gosec
	// can't see a way for this be abused
	// but please do let me know if you see otherwise
}

func encode(out io.Writer, trackPath, cachePath string, profile Profile) error {
	// prepare cache part file path
	cachePartPath := fmt.Sprintf("%s.part", cachePath)
	// prepare the command and file descriptors
	cmd, err := ffmpegCommand(trackPath, profile)
	if err != nil {
		return fmt.Errorf("generate ffmpeg command: %w", err)
	}
	pipeReader, pipeWriter := io.Pipe()
	cmd.Stdout = pipeWriter
	cmd.Stderr = pipeWriter
	// create cache part file
	cacheFile, err := os.Create(cachePartPath)
	if err != nil {
		return fmt.Errorf("writing to cache file %q: %v: %w", cachePath, err, err)
	}
	// still unsure if buffer version (cmdOutputWrite) is any better than io.Copy-based one (cmdOutputCopy)
	// initial goal here is to start streaming response asap, with smallest ttfb. more testing needed
	// -- @spijet
	//
	// start up writers for cache file and http response
	go cmdOutputWrite(out, cacheFile, pipeReader)
	// run ffmpeg
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running ffmpeg: %w", err)
	}
	// close all pipes and flush cache part file
	_ = pipeWriter.Close()
	if err := cacheFile.Sync(); err != nil {
		return fmt.Errorf("flushing %q: %w", cachePath, err)
	}
	_ = cacheFile.Close()
	// rename cache part file to mark it as valid cache file
	_ = os.Rename(cachePartPath, cachePath)
	return nil
}

// cacheKey generates the filename for the new transcode save
func cacheKey(sourcePath string, profileName string, profile Profile) string {
	return fmt.Sprintf("%x-%s-%dk.%s",
		xxhash.Sum64String(sourcePath), profileName, profile.Bitrate, profile.Format,
	)
}

type (
	OnInvalidProfileFunc func() error
	OnCacheHitFunc       func(Profile, string) error
	OnCacheMissFunc      func(Profile) (io.Writer, error)
)

type Options struct {
	TrackPath        string
	TrackBitrate     int
	CachePath        string
	ProfileName      string
	PreferredBitrate int
	OnInvalidProfile OnInvalidProfileFunc
	OnCacheHit       OnCacheHitFunc
	OnCacheMiss      OnCacheMissFunc
}

func Encode(opts Options) error {
	profile, ok := Profiles()[opts.ProfileName]
	if !ok {
		return opts.OnInvalidProfile()
	}
	switch {
	case opts.PreferredBitrate != 0 && opts.PreferredBitrate >= opts.TrackBitrate:
		log.Printf("not transcoding, requested bitrate larger or equal to track bitrate\n")
		return opts.OnInvalidProfile()
	case opts.PreferredBitrate != 0 && opts.PreferredBitrate < opts.TrackBitrate:
		profile.Bitrate = opts.PreferredBitrate
		log.Printf("transcoding according to client request of %dk \n", profile.Bitrate)
	case opts.PreferredBitrate == 0 && profile.Bitrate >= opts.TrackBitrate:
		log.Printf("not transcoding, profile bitrate larger or equal to track bitrate\n")
		return opts.OnInvalidProfile()
	case opts.PreferredBitrate == 0 && profile.Bitrate < opts.TrackBitrate:
		log.Printf("transcoding according to transcoding profile of %dk\n", profile.Bitrate)
	}
	cacheKey := cacheKey(opts.TrackPath, opts.ProfileName, profile)
	cachePath := path.Join(opts.CachePath, cacheKey)
	if fileExists(cachePath) {
		return opts.OnCacheHit(profile, cachePath)
	}
	writer, err := opts.OnCacheMiss(profile)
	if err != nil {
		return fmt.Errorf("starting cache serve: %w", err)
	}
	if err := encode(writer, opts.TrackPath, cachePath, profile); err != nil {
		return fmt.Errorf("starting transcode: %w", err)
	}
	return nil
}
