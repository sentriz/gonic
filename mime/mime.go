package mime

import (
	gomime "mime"
	"strings"
)

//nolint:gochecknoinits
func init() {
	_ = gomime.AddExtensionType(".mp3", "audio/mpeg")
	_ = gomime.AddExtensionType(".flac", "audio/x-flac")
	_ = gomime.AddExtensionType(".aac", "audio/x-aac")
	_ = gomime.AddExtensionType(".m4a", "audio/m4a")
	_ = gomime.AddExtensionType(".m4b", "audio/m4b")
	_ = gomime.AddExtensionType(".ogg", "audio/ogg")
	_ = gomime.AddExtensionType(".opus", "audio/ogg")
	_ = gomime.AddExtensionType(".wma", "audio/x-ms-wma")
}

func FromAudioExtension(ext string) string {
	if ext == "" {
		return ""
	}
	mime := gomime.TypeByExtension("." + ext)
	if !strings.HasPrefix(mime, "audio/") {
		return ""
	}
	return mime
}
