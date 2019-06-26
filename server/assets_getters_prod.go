// +build embed

package server

import (
	"bytes"
	"io"
	"time"
)

func (_ *Assets) Find(path string) (time.Time, io.ReadSeeker, error) {
	asset, ok := AssetBytes[path]
	if !ok {
		return time.Time{}, nil, ErrAssetNotFound
	}
	reader := bytes.NewReader(asset.Bytes)
	return asset.ModTime, reader, nil
}

func (_ *Assets) FindBytes(path string) (time.Time, []byte, error) {
	asset, ok := AssetBytes[path]
	if !ok {
		return time.Time{}, nil, ErrAssetNotFound
	}
	return asset.ModTime, asset.Bytes, nil
}
