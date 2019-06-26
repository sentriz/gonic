package server

import "errors"

var ErrAssetNotFound = errors.New("asset not found")

type Assets struct {
	BasePath string
}
