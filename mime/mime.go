package mime

func FromExtension(ext string) string {
	switch ext {
	case "mp3":
		return "audio/mpeg"
	case "flac":
		return "audio/x-flac"
	case "aac":
		return "audio/x-aac"
	case "m4a":
		return "audio/m4a"
	case "m4b":
		return "audio/m4b"
	case "ogg":
		return "audio/ogg"
	case "opus":
		return "audio/ogg"
	case "wma":
		return "audio/x-ms-wma"
	default:
		return ""
	}
}
