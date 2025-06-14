package sandbox

import (
	"log"
	"os/exec"

	"golang.org/x/sys/unix"
)

type Sandbox struct{}

func Init() Sandbox {
	box := Sandbox{}
	if err := unix.PledgePromises("stdio rpath cpath wpath flock inet unveil dns proc exec fattr"); err != nil {
		log.Fatalf("failed to pledge: %v", err)
	}
	// find the transcoding and jukebox paths before doing any other unveils
	// otherwise looking for it will fail
	ffmpegPath, ffmpegErr := exec.LookPath("ffmpeg")
	mpvPath, mpvErr := exec.LookPath("mpv")
	if ffmpegErr == nil || mpvErr == nil {
		if ffmpegErr == nil {
			box.ExecPath(ffmpegPath)
		}
		if mpvErr == nil {
			box.ExecPath(mpvPath)
		}
	} else {
		// we can restrict our permissions
		if err := unix.PledgePromises("stdio rpath cpath wpath flock inet unveil dns"); err != nil {
			log.Fatalf("failed to pledge: %v", err)
		}
	}
	// needed to enable certificate validation
	box.ReadOnlyFile("/etc/ssl/cert.pem")
	return box
}

func (box *Sandbox) ExecPath(path string) {
	if err := unix.Unveil(path, "rx"); err != nil {
		log.Fatalf("failed to unveil exec for %s: %v", path, err)
	}
}

func (box *Sandbox) ReadOnlyDir(path string) {
	if err := unix.Unveil(path, "r"); err != nil {
		log.Fatalf("failed to unveil read for %s: %v", path, err)
	}
}

func (box *Sandbox) ReadOnlyFile(path string) {
	if err := unix.Unveil(path, "r"); err != nil {
		log.Fatalf("failed to unveil read for %s: %v", path, err)
	}
}

func (box *Sandbox) ReadWriteCreateDir(path string) {
	if err := unix.Unveil(path, "rwc"); err != nil {
		log.Fatalf("failed to unveil read/write/create for %s: %v", path, err)
	}
}

func (box *Sandbox) ReadWriteCreateFile(path string) {
	if err := unix.Unveil(path, "rwc"); err != nil {
		log.Fatalf("failed to unveil read/write/create for %s: %v", path, err)
	}
}

func (box *Sandbox) AllPathsAdded() {
	if err := unix.UnveilBlock(); err != nil {
		log.Fatalf("failed to finalize unveil: %v", err)
	}
}
