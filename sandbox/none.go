//go:build !(openbsd || linux)

package sandbox

type Sandbox struct{}

func Init() Sandbox {
	return Sandbox{}
}

func (box *Sandbox) ReadOnlyDir(path string) {
}

func (box *Sandbox) ReadOnlyFile(path string) {
}

func (box *Sandbox) ReadWriteCreateDir(path string) {
}

func (box *Sandbox) ReadWriteCreateFile(path string) {
}

func (box *Sandbox) AllPathsAdded() {
}
