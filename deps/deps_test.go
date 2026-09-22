//go:build !nowasm && ffprobe

package deps

import "testing"

func TestTagReaderCanRead(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"track.flac":   true,
		"video.mkv":    true,
		"video.webm":   true,
		"video.mp4":    true,
		"video.avi":    true,
		"document.pdf": false,
	}

	for path, want := range tests {
		if got := TagReader.CanRead(path); got != want {
			t.Errorf("TagReader.CanRead(%q) = %v, want %v", path, got, want)
		}
	}
}
