package coverresolve

import (
	"testing"
)

func TestIsCover(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"JPEG file", "Image.jpg", true},
		{"JPEG file", "image.jpg", true},
		{"PNG file", "picture.png", true},
		{"BMP file", "photo.bmp", true},
		{"GIF file", "animation.gif", true},
		{"Non-image file", "document.pdf", false},
		{"Empty file name", "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsCover(test.filename)
			if result != test.expected {
				t.Errorf("Expected IsCover(%q) to be %v, but got %v", test.filename, test.expected, result)
			}
		})
	}
}

func TestSelectCover(t *testing.T) {

	tests := []struct {
		name     string
		covers   []string
		expected string
	}{
		{
			name:     "Empty covers slice",
			covers:   []string{},
			expected: "",
		},
		{
			name:     "Covers without keywords or numbers case sensitive",
			covers:   []string{"Cover1.jpg", "cover2.png"},
			expected: "Cover1.jpg",
		},
		{
			name:     "Covers without keywords or numbers",
			covers:   []string{"cover1.jpg", "cover2.png"},
			expected: "cover1.jpg",
		},
		{
			name:     "Covers with keywords and numbers",
			covers:   []string{"cover12.jpg", "cover2.png", "special_cover1.jpg"},
			expected: "special_cover1.jpg",
		},
		{
			name:     "Covers with keywords but without numbers",
			covers:   []string{"cover12.jpg", "cover_keyword.png"},
			expected: "cover_keyword.png",
		},
		{
			name:     "Covers without keywords but with numbers",
			covers:   []string{"cover1.jpg", "cover12.png"},
			expected: "cover1.jpg",
		},
		{
			name:     "Covers with same highest score",
			covers:   []string{"cover1.jpg", "cover2.jpg", "cover_special.jpg"},
			expected: "cover_special.jpg",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Mock the DefaultScoreRules
			result := SelectCover(test.covers)
			if result != test.expected {
				t.Errorf("Expected SelectCover(%v) to be %q, but got %q", test.covers, test.expected, result)
			}
		})
	}
}
