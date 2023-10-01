//nolint:gochecknoglobals
package mime

import (
	"log"
	stdmime "mime"
	"strings"
)

//nolint:gochecknoinits
func init() {
	for ext, mime := range supportedAudioTypes {
		if err := stdmime.AddExtensionType(ext, mime); err != nil {
			log.Fatalf("adding audio type mime for ext %q: %v", ext, err)
		}
	}
}

var (
	TypeByExtension = stdmime.TypeByExtension
	ParseMediaType  = stdmime.ParseMediaType
	FormatMediaType = stdmime.FormatMediaType
)

func TypeByAudioExtension(ext string) string {
	if _, ok := supportedAudioTypes[strings.ToLower(ext)]; !ok {
		return ""
	}
	return stdmime.TypeByExtension(ext)
}

var supportedAudioTypes = map[string]string{
	".mp3":  "audio/mpeg",
	".flac": "audio/x-flac",
	".aac":  "audio/x-aac",
	".m4a":  "audio/m4a",
	".m4b":  "audio/m4b",
	".ogg":  "audio/ogg",
	".opus": "audio/ogg",
	".wma":  "audio/x-ms-wma",
	".wav":  "audio/x-wav",
	".wv":   "audio/x-wavpack",
}
