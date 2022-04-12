package transcode

import (
	"context"
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

func (*FFmpegTranscoder) Transcode(ctx context.Context, profile Profile, in string) (io.ReadCloser, error) {
	name, args, err := parseProfile(profile, in)
	if err != nil {
		return nil, fmt.Errorf("split command: %w", err)
	}

	preader, pwriter := io.Pipe()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = pwriter
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting cmd: %w", err)
	}

	go func() {
		_ = pwriter.CloseWithError(cmd.Wait())
	}()

	return preader, nil
}
