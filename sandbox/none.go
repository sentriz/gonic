//go:build !openbsd
// +build !openbsd

package sandbox

func Init() {
}

func ReadOnlyPath(path string) {
}

func ReadWritePath(path string) {
}

func ReadWriteCreatePath(path string) {
}

func AllPathsAdded() {
}
