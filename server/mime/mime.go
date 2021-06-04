package mime

// this package is at such a high level in the hierarchy because
// it's used by both `server/db` and `server/scanner`

func FromExtension(ext string) (string, bool) {
	types := map[string]string{
		"mp3":  "audio/mpeg",
		"flac": "audio/x-flac",
		"aac":  "audio/x-aac",
		"m4a":  "audio/m4a",
		"m4b":  "audio/m4b",
		"ogg":  "audio/ogg",
		"opus": "audio/ogg",
		"wma":  "audio/x-ms-wma",
	}
	v, ok := types[ext]
	return v, ok
}
