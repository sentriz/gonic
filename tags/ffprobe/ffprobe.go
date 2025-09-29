package ffprobe

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.senan.xyz/gonic/tags"
)

var _ tags.Reader = Reader{}

type Reader struct{}

func (Reader) CanRead(absPath string) bool {
	switch ext := strings.ToLower(filepath.Ext(absPath)); ext {
	case ".3ga", ".3gp", ".669", ".aa3", ".aac", ".aif", ".aiff", ".aifc", ".ape", ".caf", ".dsf", ".f4a", ".f4b", ".flac", ".it", ".m4a", ".m4b", ".m4r", ".mka", ".mkv", ".mod", ".mov", ".mp1", ".mp2", ".mp3", ".mp4", ".mpc", ".mpp", ".oga", ".ogg", ".oma", ".opus", ".ra", ".rf64", ".rm", ".s3m", ".sph", ".spx", ".stm", ".tak", ".tta", ".wav", ".webm", ".w64", ".wma", ".wv", ".asf":
		return true
	}
	return false
}

func (Reader) Read(absPath string) (tags.Properties, map[string][]string, error) {
	out, err := exec.Command("ffprobe", "-hide_banner", "-v", "0", "-i", absPath, "-show_entries", "format", "-of", "json").Output()
	if err != nil {
		return tags.Properties{}, nil, fmt.Errorf("output: %w", err)
	}

	var format struct {
		Format struct {
			Duration string            `json:"duration"`
			BitRate  string            `json:"bit_rate"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &format); err != nil {
		return tags.Properties{}, nil, fmt.Errorf("read json: %w", err)
	}

	durationSecs, _ := strconv.ParseFloat(format.Format.Duration, 64)
	bitRateBitsPerSec, _ := strconv.Atoi(format.Format.BitRate)

	var tgs = map[string][]string{}
	for k, vs := range format.Format.Tags {
		switch k {
		case "OK":
			continue
		}
		tgs[k] = strings.Split(vs, ";")
	}

	props := tags.Properties{
		Length:  time.Duration(durationSecs) * time.Second,
		Bitrate: uint(bitRateBitsPerSec / 1000),
	}

	return props, tgs, nil
}

func (Reader) ReadCover(absPath string) ([]byte, error) {
	out, err := exec.Command("ffmpeg", "-i", absPath, "-map", "0:v", "-c", "copy", "-f", "image2pipe", "-").Output()
	if err != nil {
		return nil, fmt.Errorf("exec: %w", err)
	}
	return out, nil
}
