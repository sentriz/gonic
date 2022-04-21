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

var ErrFFmpegExit = fmt.Errorf("ffmpeg exited with non 0 status code")

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
	if err := cmd.Wait(); err != nil && !errors.As(err, &exitErr) {
		return fmt.Errorf("waiting cmd: %w", err)
	}
	if code := cmd.ProcessState.ExitCode(); code > 1 {
		return fmt.Errorf("%w: %d", ErrFFmpegExit, code)
	}
	return nil
}
