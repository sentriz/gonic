package transcode

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.senan.xyz/gonic/iout"
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

func (t *CachingTranscoder) Transcode(ctx context.Context, profile Profile, in string) (io.ReadCloser, error) {
	if err := os.MkdirAll(t.cachePath, perm^0111); err != nil {
		return nil, fmt.Errorf("make cache path: %w", err)
	}

	name, args, err := parseProfile(profile, in)
	if err != nil {
		return nil, fmt.Errorf("split command: %w", err)
	}

	key := cacheKey(name, args)
	path := filepath.Join(t.cachePath, key)

	cf, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open cache file: %w", err)
	}
	if i, err := cf.Stat(); err == nil && i.Size() > 0 {
		return cf, nil
	}

	out, err := t.transcoder.Transcode(ctx, profile, in)
	if err != nil {
		return nil, fmt.Errorf("internal transcode: %w", err)
	}

	return iout.NewTeeCloser(out, cf), nil
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
