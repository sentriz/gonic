package paths

import (
	"fmt"
	"path/filepath"
	"strings"
)

const sep = "->"

type MusicPaths []MusicPath

func (mps MusicPaths) String() string {
	var strs []string
	for _, path := range mps {
		strs = append(strs, path.String())
	}
	return strings.Join(strs, ", ")
}

func (mps *MusicPaths) Set(value string) error {
	alias, path, ok := strings.Cut(value, sep)
	if !ok {
		*mps = append(*mps, MusicPath{
			Path: filepath.Clean(strings.TrimSpace(value)),
		})
		return nil
	}
	*mps = append(*mps, MusicPath{
		Alias: strings.TrimSpace(alias),
		Path:  filepath.Clean(strings.TrimSpace(path)),
	})
	return nil
}

func (mps MusicPaths) Paths() []string {
	var paths []string
	for _, mp := range mps {
		paths = append(paths, mp.Path)
	}
	return paths
}

type MusicPath struct {
	Alias string
	Path  string
}

func (mp MusicPath) String() string {
	if mp.Alias == "" {
		return mp.Path
	}
	return fmt.Sprintf("%s %s %s", mp.Alias, sep, mp.Path)
}

func (mp MusicPath) DisplayAlias() string {
	if mp.Alias == "" {
		return filepath.Base(mp.Path)
	}
	return mp.Alias
}
