package taglib

import (
	"cmp"
	"fmt"
	"path/filepath"
	"strings"

	"go.senan.xyz/gonic/tags"
	"go.senan.xyz/taglib"
)

var _ tags.Reader = Reader{}

type Reader struct{}

func (Reader) CanRead(absPath string) bool {
	switch ext := strings.ToLower(filepath.Ext(absPath)); ext {
	case ".mp3", ".flac", ".aac", ".m4a", ".m4b", ".ogg", ".opus", ".wma", ".wav", ".wv", ".ape":
		return true
	}
	return false
}

func (Reader) Read(absPath string) (tags.Properties, tags.Tags, error) {
	tp, err := taglib.ReadProperties(absPath)
	if err != nil {
		return tags.Properties{}, nil, fmt.Errorf("read properties: %w", err)
	}

	tag, err := taglib.ReadTags(absPath)
	if err != nil {
		return tags.Properties{}, nil, fmt.Errorf("read tags: %w", err)
	}

	return tags.Properties{
		Length:     tp.Length,
		Bitrate:    tp.BitRate,
		Channels:   tp.Channels,
		SampleRate: tp.SampleRate,
		BitDepth:   tp.BitDepth,
		Codec:      normCodec(tp),
		HasCover:   len(tp.Images) > 0,
	}, tag, nil
}

func (Reader) ReadCover(absPath string) ([]byte, error) {
	return taglib.ReadImage(absPath)
}

// normCodec maps taglib's format/inner codec onto gonic's ffmpeg-flavoured canon
func normCodec(tp taglib.Properties) string {
	switch codec := cmp.Or(tp.InnerCodec, tp.Format); codec {
	case "mpeg":
		return "mp3"
	case "wma1":
		return "wmav1"
	case "wma2":
		return "wmav2"
	case "wma9pro":
		return "wmapro"
	case "wma9lossless":
		return "wmalossless"
	default:
		return codec
	}
}
