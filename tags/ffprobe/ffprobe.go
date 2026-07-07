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

func (Reader) Read(absPath string) (tags.Properties, tags.Tags, error) {
	out, err := exec.Command("ffprobe", "-hide_banner", "-v", "0", "-i", absPath, "-show_entries", "format:stream=codec_type,codec_name,channels,sample_rate,bits_per_raw_sample,bits_per_sample:stream_tags", "-of", "json").Output()
	if err != nil {
		return tags.Properties{}, nil, fmt.Errorf("output: %w", err)
	}

	var d struct {
		Streams []struct {
			CodecType        string            `json:"codec_type"`
			CodecName        string            `json:"codec_name"`
			Channels         int               `json:"channels"`
			SampleRate       string            `json:"sample_rate"`
			BitsPerRawSample string            `json:"bits_per_raw_sample"`
			BitsPerSample    int               `json:"bits_per_sample"`
			Tags             map[string]string `json:"tags"`
		} `json:"streams"`
		Format struct {
			Duration string            `json:"duration"`
			BitRate  string            `json:"bit_rate"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &d); err != nil {
		return tags.Properties{}, nil, fmt.Errorf("read json: %w", err)
	}

	durationSecs, _ := strconv.ParseFloat(d.Format.Duration, 64)
	bitRateBitsPerSec, _ := strconv.Atoi(d.Format.BitRate)

	props := tags.Properties{
		Length:  time.Duration(durationSecs) * time.Second,
		Bitrate: uint(bitRateBitsPerSec / 1000),
	}

	var tgs = map[string][]string{}
	var gotAudio bool
	for _, s := range d.Streams {
		switch s.CodecType {
		case "video":
			props.HasCover = true
		case "audio":
			if gotAudio {
				continue // first audio stream wins
			}
			gotAudio = true
			for k, vs := range s.Tags {
				tgs[k] = strings.Split(vs, ";")
			}
			props.Codec, _, _ = strings.Cut(s.CodecName, "_") // pcm_s16le, dsd_lsbf, ... -> pcm, dsd
			props.Channels = uint(s.Channels)
			sampleRate, _ := strconv.Atoi(s.SampleRate)
			props.SampleRate = uint(sampleRate)
			bitDepth, _ := strconv.Atoi(s.BitsPerRawSample) // unset (0) for lossy
			props.BitDepth = uint(max(bitDepth, s.BitsPerSample))
		}
	}
	for k, vs := range d.Format.Tags {
		switch k {
		case "OK":
			continue
		}
		tgs[k] = strings.Split(vs, ";")
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
