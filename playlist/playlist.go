package playlist

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrInvalidPathFormat = errors.New("invalid path format")
var ErrInvalidBasePath = errors.New("invalid base path")
var ErrNoUserPrefix = errors.New("no user prefix")

const (
	extM3U  = ".m3u"
	extM3U8 = ".m3u8"
)

type Store struct {
	basePath string
	mu       sync.Mutex
}

func NewStore(basePath string) (*Store, error) {
	if basePath == "" {
		return nil, ErrInvalidBasePath

	}

	// sanity check layout, just in case someone tries to use an existing folder
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("sanity checking: reading dir: %w", err)
	}
	var found string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := userIDFromPath(entry.Name()); err != nil {
			found = entry.Name()
			break
		}
	}
	if found != "" {
		return nil, fmt.Errorf("sanity checking: %w: item %q in playlists directory is not a user id. see wiki for details on layout of the playlists dir", ErrNoUserPrefix, found)
	}

	return &Store{
		basePath: basePath,
	}, nil
}

type Playlist struct {
	UpdatedAt time.Time
	UserID    int
	Name      string
	Comment   string
	Items     []string
	IsPublic  bool
}

func NewPath(userID int, playlistName string) string {
	playlistName = safeFilename(playlistName)
	if playlistName == "" {
		playlistName = "pl"
	}
	playlistName = fmt.Sprintf("%s-%d%s", playlistName, time.Now().UnixMilli(), extM3U)
	return filepath.Join(fmt.Sprint(userID), playlistName)
}

// List finds playlist items in s.basePath.
// the expected format is <base path>/<user id>/**/<playlist name>.m3u
func (s *Store) List() ([]string, error) {
	var relPaths []string
	return relPaths, filepath.WalkDir(s.basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case extM3U, extM3U8:
		default:
			return nil
		}
		relPath, _ := filepath.Rel(s.basePath, path)
		relPaths = append(relPaths, relPath)
		return nil
	})
}

const (
	attrPrefix   = "#GONIC-"
	attrName     = "NAME"
	attrCommment = "COMMENT"
	attrIsPublic = "IS-PUBLIC"
)

func encodeAttr(name, value string) string {
	return fmt.Sprintf("%s%s:%s", attrPrefix, name, strconv.Quote(value))
}
func decodeAttr(line string) (name, value string) {
	if !strings.HasPrefix(line, attrPrefix) {
		return "", ""
	}
	prefixAndName, rawValue, _ := strings.Cut(line, ":")
	name = strings.TrimPrefix(prefixAndName, attrPrefix)
	value, _ = strconv.Unquote(rawValue)
	return name, value
}

func (s *Store) Read(relPath string) (*Playlist, error) {
	defer lock(&s.mu)()

	absPath := filepath.Join(s.basePath, relPath)
	stat, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat m3u: %w", err)
	}

	var playlist Playlist
	playlist.UpdatedAt = stat.ModTime()

	playlist.UserID, err = userIDFromPath(relPath)
	if err != nil {
		return nil, fmt.Errorf("convert id to str: %w", err)
	}

	playlist.Name = strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))

	file, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("open m3u: %w", err)
	}
	defer file.Close()

	for sc := bufio.NewScanner(file); sc.Scan(); {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		switch name, value := decodeAttr(line); name {
		case attrName:
			playlist.Name = value
		case attrCommment:
			playlist.Comment = value
		case attrIsPublic:
			playlist.IsPublic, _ = strconv.ParseBool(value)
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		playlist.Items = append(playlist.Items, line)
	}

	return &playlist, nil
}

func (s *Store) Write(relPath string, playlist *Playlist) error {
	defer lock(&s.mu)()

	absPath := filepath.Join(s.basePath, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0777); err != nil {
		return fmt.Errorf("make m3u base dir: %w", err)
	}
	file, err := os.OpenFile(absPath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("create m3u: %w", err)
	}
	defer file.Close()

	if err := os.Chtimes(absPath, time.Time{}, playlist.UpdatedAt); err != nil {
		return fmt.Errorf("touch m3u: %w", err)
	}

	var existingComments []string
	for sc := bufio.NewScanner(file); sc.Scan(); {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, attrPrefix) {
			continue
		}
		if strings.HasPrefix(line, "#") {
			existingComments = append(existingComments, sc.Text())
		}
	}

	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("seek m3u: %w", err)
	}
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("truncate m3u: %w", err)
	}

	for _, line := range existingComments {
		fmt.Fprintln(file, line)
	}
	fmt.Fprintln(file, encodeAttr(attrName, playlist.Name))
	fmt.Fprintln(file, encodeAttr(attrCommment, playlist.Comment))
	fmt.Fprintln(file, encodeAttr(attrIsPublic, fmt.Sprint(playlist.IsPublic)))
	for _, line := range playlist.Items {
		fmt.Fprintln(file, line)
	}

	return nil
}

func (s *Store) Delete(relPath string) error {
	return os.Remove(filepath.Join(s.basePath, relPath))
}

var nonAlphaNum = regexp.MustCompile("[^a-zA-Z0-9_.]+")

func safeFilename(filename string) string {
	filename = nonAlphaNum.ReplaceAllString(filename, "")
	return filename
}

func firstPathEl(path string) string {
	path = strings.TrimPrefix(path, string(filepath.Separator))
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func userIDFromPath(relPath string) (int, error) {
	return strconv.Atoi(firstPathEl(relPath))
}

func lock(mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}
