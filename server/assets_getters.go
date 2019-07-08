package server

import (
	"bytes"
	"errors"
	"io"
	"time"
)

var errAssetNotFound = errors.New("asset not found")

func findAsset(path string) (time.Time, io.ReadSeeker, error) {
	asset, ok := assetBytes[path]
	if !ok {
		return time.Time{}, nil, errAssetNotFound
	}
	reader := bytes.NewReader(asset.Bytes)
	return asset.ModTime, reader, nil
}

func findAssetBytes(path string) (time.Time, []byte, error) {
	asset, ok := assetBytes[path]
	if !ok {
		return time.Time{}, nil, errAssetNotFound
	}
	return asset.ModTime, asset.Bytes, nil
}
