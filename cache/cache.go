package cache

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// DirCache is an LRU file cache backed by a directory. Callers that write new
// files must hold an RLock for the duration of the write so that Eject cannot
// delete a file that is still being written.
type DirCache struct {
	path    string
	limitMB int
	mu      sync.RWMutex
}

func New(path string, limitMB int) *DirCache {
	return &DirCache{path: path, limitMB: limitMB}
}

func (c *DirCache) Path() string { return c.path }

func (c *DirCache) RLock()   { c.mu.RLock() }
func (c *DirCache) RUnlock() { c.mu.RUnlock() }

// Eject removes the least-recently-used files until the cache is within its
// size limit. It holds a write lock for its duration, blocking concurrent
// reads.
func (c *DirCache) Eject() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	type file struct {
		path string
		info os.FileInfo
	}

	var files []file
	var total int64

	err := filepath.WalkDir(c.path, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !de.IsDir() {
			info, err := de.Info()
			if err != nil {
				return fmt.Errorf("stat cache file: %w", err)
			}
			files = append(files, file{path, info})
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk cache path for eject: %w", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].info.ModTime().Before(files[j].info.ModTime())
	})

	for total > int64(c.limitMB)*1024*1024 {
		curFile := files[0]
		files = files[1:]
		total -= curFile.info.Size()
		if err := os.Remove(curFile.path); err != nil {
			return fmt.Errorf("remove cache file: %w", err)
		}
	}

	return nil
}
