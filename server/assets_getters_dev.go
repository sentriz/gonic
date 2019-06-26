// +build !embed

package server

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

func (a *Assets) Find(path string) (time.Time, io.ReadSeeker, error) {
	fullPath := filepath.Join(a.BasePath, path)
	info, err := os.Stat(fullPath)
	if err != nil {
		return time.Time{}, nil, errors.Wrap(err, "statting asset")
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return time.Time{}, nil, errors.Wrapf(ErrAssetNotFound, "%v", err)
	}
	return info.ModTime(), file, nil
}

func (a *Assets) FindBytes(path string) (time.Time, []byte, error) {
	fullPath := filepath.Join(a.BasePath, path)
	info, err := os.Stat(fullPath)
	if err != nil {
		return time.Time{}, nil, errors.Wrap(err, "statting asset")
	}
	file, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return time.Time{}, nil, errors.Wrapf(ErrAssetNotFound, "%v", err)
	}
	return info.ModTime(), file, nil
}
