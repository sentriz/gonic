package transcode

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const perm = 0644

type CachingTranscoder struct {
	cachePath  string
	transcoder Transcoder
}

var _ Transcoder = (*CachingTranscoder)(nil)

func NewCachingTranscoder(t Transcoder, cachePath string) *CachingTranscoder {
	return &CachingTranscoder{transcoder: t, cachePath: cachePath}
}

func (t *CachingTranscoder) Transcode(ctx context.Context, profile Profile, in string, out io.Writer) error {
	if err := os.MkdirAll(t.cachePath, perm^0111); err != nil {
		return fmt.Errorf("make cache path: %w", err)
	}

	name, args, err := parseProfile(profile, in)
	if err != nil {
		return fmt.Errorf("split command: %w", err)
	}

	key := cacheKey(name, args)
	path := filepath.Join(t.cachePath, key)

	cf, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("open cache file: %w", err)
	}
	defer cf.Close()

	if i, err := cf.Stat(); err == nil && i.Size() > 0 {
		_, _ = io.Copy(out, cf)
		return nil
	}

	if err := t.transcoder.Transcode(ctx, profile, in, io.MultiWriter(out, cf)); err != nil {
		os.Remove(path)
		return fmt.Errorf("internal transcode: %w", err)
	}

	return nil
}

func cacheKey(cmd string, args []string) string {
	// the cache is invalid whenever transcode command (which includes the
	// absolute filepath, bit rate args, replay gain args, etc.) changes
	sum := md5.New()
	_, _ = io.WriteString(sum, cmd)
	for _, arg := range args {
		_, _ = io.WriteString(sum, arg)
	}
	return fmt.Sprintf("%x", sum.Sum(nil))
}
