package ctrlsubsonic

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/fileutil"
)

type albumNFO struct {
	XMLName xml.Name `xml:"album"`
	Reviews []string `xml:"review"`
}

const (
	albumNFOFilename = "album.nfo"
	albumNFOMaxBytes = 1 << 20 // 1 MiB
)

func albumNotesFromNFO(album db.Album) (string, error) {
	albumDir := album.AbsPath()
	if albumDir == "" {
		return "", nil
	}

	file, err := os.Open(filepath.Join(albumDir, albumNFOFilename)) //nolint:gosec // album path comes from scanner-populated db
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("open nfo: %w", err)
	}
	defer file.Close()

	return parseAlbumNFOReview(file)
}

func parseAlbumNFOReview(r io.Reader) (string, error) {
	limited := io.LimitReader(r, albumNFOMaxBytes)
	var nfo albumNFO
	if err := xml.NewDecoder(fileutil.StripBOM(limited)).Decode(&nfo); err != nil {
		return "", fmt.Errorf("decode nfo xml: %w", err)
	}

	// Jellyfin's BaseNfoParser evaluates elements sequentially, so the last non-empty <review> wins.
	for i := len(nfo.Reviews) - 1; i >= 0; i-- {
		review := normalizeNFOText(nfo.Reviews[i])
		if review != "" {
			return review, nil
		}
	}
	return "", nil
}

func normalizeNFOText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}
