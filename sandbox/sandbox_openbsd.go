package sandbox

import (
	"log"
	"os/exec"

	"golang.org/x/sys/unix"
)

func Init() {
	if err := unix.PledgePromises("stdio rpath cpath wpath flock inet unveil dns proc exec fattr"); err != nil {
		log.Fatalf("failed to pledge: %v", err)
	}
	// find the transcoding and jukebox paths before doing any other unveils
	// otherwise looking for it will fail
	ffmpegPath, ffmpegErr := exec.LookPath("ffmpeg")
	mpvPath, mpvErr := exec.LookPath("mpv")
	if ffmpegErr == nil || mpvErr == nil {
		if ffmpegErr == nil {
			ExecPath(ffmpegPath)
		}
		if mpvErr == nil {
			ExecPath(mpvPath)
		}
	} else {
		// we can restrict our permissions
		if err := unix.PledgePromises("stdio rpath cpath wpath flock inet unveil dns"); err != nil {
			log.Fatalf("failed to pledge: %v", err)
		}
	}
	// needed to enable certificate validation
	ReadOnlyPath("/etc/ssl/cert.pem")
}

func ExecPath(path string) {
	if err := unix.Unveil(path, "rx"); err != nil {
		log.Fatalf("failed to unveil exec for %s: %v", path, err)
	}
}

func ReadOnlyPath(path string) {
	if err := unix.Unveil(path, "r"); err != nil {
		log.Fatalf("failed to unveil read for %s: %v", path, err)
	}
}

func ReadWritePath(path string) {
	if err := unix.Unveil(path, "rw"); err != nil {
		log.Fatalf("failed to unveil read/write for %s: %v", path, err)
	}
}

func ReadWriteCreatePath(path string) {
	if err := unix.Unveil(path, "rwc"); err != nil {
		log.Fatalf("failed to unveil read/write/create for %s: %v", path, err)
	}
}

func AllPathsAdded() {
	if err := unix.UnveilBlock(); err != nil {
		log.Fatalf("failed to finalize unveil: %v", err)
	}
}
