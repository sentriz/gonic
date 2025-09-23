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
	case ".mp3", ".flac", ".aac", ".m4a", ".m4b", ".ogg", ".opus", ".wma", ".wav", ".wv":
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
