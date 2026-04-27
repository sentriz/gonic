package ctrlsubsonic

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type CoverCache struct {
	path      string
	limitMB   int
	cleanLock sync.RWMutex
}

func NewCoverCache(path string, limitMB int) *CoverCache {
	return &CoverCache{path: path, limitMB: limitMB}
}

func (c *CoverCache) CacheEject() error {
	c.cleanLock.Lock()
	defer c.cleanLock.Unlock()

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
				return fmt.Errorf("stat cover cache file: %w", err)
			}
			files = append(files, file{path, info})
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk cover cache path for eject: %w", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].info.ModTime().Before(files[j].info.ModTime())
	})

	for total > int64(c.limitMB)*1024*1024 {
		curFile := files[0]
		files = files[1:]
		total -= curFile.info.Size()
		if err := os.Remove(curFile.path); err != nil {
			return fmt.Errorf("remove cover cache file: %w", err)
		}
	}

	return nil
}
