package sandbox

import (
	"log"
	"os"

	"github.com/landlock-lsm/go-landlock/landlock"
)

type Sandbox struct {
	paths []landlock.Rule
}

func Init() Sandbox {
	return Sandbox{make([]landlock.Rule, 0)}
}

func (box *Sandbox) ExecPath(path string) {
	// landlock does not currently provide anything for us here
}

func (box *Sandbox) ReadOnlyDir(path string) {
	box.paths = append(box.paths, landlock.RODirs(path))
}

func (box *Sandbox) ReadOnlyFile(path string) {
	box.paths = append(box.paths, landlock.ROFiles(path))
}

func (box *Sandbox) ReadWriteCreateDir(path string) {
	box.paths = append(box.paths, landlock.RWDirs(path))
}

func (box *Sandbox) ReadWriteCreateFile(path string) {
	// landlock requires the file to already exist
	// so create it if it wasn't already present
	_, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		file, err := os.Create(path)
		if err != nil {
			log.Fatal("Could not create file for landlock:", err)
		} else {
			file.Close()
		}
	}
	box.paths = append(box.paths, landlock.RWFiles(path))
}

func (box *Sandbox) AllPathsAdded() {
	if err := landlock.V5.BestEffort().RestrictPaths(box.paths...); err != nil {
		log.Fatal("Could not enable landlock:", err)
	}
	// FIXME: clear paths
}
