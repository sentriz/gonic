package coverresolve

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var DefaultKeywords = []string{
	"cover",
	"folder",
	"front",
	"albumart",
	"album",
	"artist",
	"scan",
}

// Helper function to extract the number from the filename
func extractNumber(filename string) int {
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(filename, -1)
	if len(matches) == 0 {
		return 0
	}
	num, _ := strconv.Atoi(matches[0])
	return num
}

type CoverAlternative struct {
	Name  string
	Score int
}

func SelectCover(covers []string) string {
	if len(covers) == 0 {
		return ""
	}

	coverAlternatives := make([]CoverAlternative, 0)

	for _, keyword := range DefaultKeywords {
		if len(coverAlternatives) > 0 {
			break
		}

		for _, cover := range covers {
			if strings.Contains(strings.ToLower(cover), keyword) {
				coverAlternatives = append(coverAlternatives, CoverAlternative{
					Name:  cover,
					Score: 0,
				})
			}
		}
	}

	// parse the integer from the filename
	// eg. cover(1).jpg will have higher score than cover(114514).jpg
	for i := range coverAlternatives {
		coverAlternatives[i].Score -= extractNumber(coverAlternatives[i].Name)
	}

	// sort by score
	sort.Slice(coverAlternatives, func(i, j int) bool {
		return coverAlternatives[i].Score > coverAlternatives[j].Score
	})

	if len(coverAlternatives) == 0 {
		return covers[0]
	}

	return coverAlternatives[0].Name
}

func IsCover(name string) bool {
	for _, ext := range []string{"jpg", "jpeg", "png", "bmp", "gif"} {
		if strings.HasSuffix(strings.ToLower(name), "."+ext) {
			return true
		}
	}
	return false
}
