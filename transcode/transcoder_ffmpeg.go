package transcode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

type FFmpegTranscoder struct{}

var _ Transcoder = (*FFmpegTranscoder)(nil)

func NewFFmpegTranscoder() *FFmpegTranscoder {
	return &FFmpegTranscoder{}
}

var (
	ErrFFmpegKilled = fmt.Errorf("ffmpeg was killed early")
	ErrFFmpegExit   = fmt.Errorf("ffmpeg exited with non 0 status code")
)

func (*FFmpegTranscoder) Transcode(ctx context.Context, profile Profile, in string, out io.Writer) error {
	name, args, err := parseProfile(profile, in)
	if err != nil {
		return fmt.Errorf("split command: %w", err)
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = out

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting cmd: %w", err)
	}

	var exitErr *exec.ExitError

	switch err := cmd.Wait(); {
	case errors.As(err, &exitErr):
		return fmt.Errorf("waiting cmd: %w: %w", err, ErrFFmpegKilled)
	case err != nil:
		return fmt.Errorf("waiting cmd: %w", err)
	}
	if code := cmd.ProcessState.ExitCode(); code > 1 {
		return fmt.Errorf("%w: %d", ErrFFmpegExit, code)
	}
	return nil
}
