package transcode

import (
	"context"
	"fmt"
	"io"
	"os"
)

type NoneTranscoder struct{}

var _ Transcoder = (*NoneTranscoder)(nil)

func NewNoneTranscoder() *NoneTranscoder {
	return &NoneTranscoder{}
}

func (*NoneTranscoder) Transcode(ctx context.Context, _ Profile, in string, out io.Writer) error {
	file, err := os.Open(in)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()
	if _, err := io.Copy(out, file); err != nil {
		return fmt.Errorf("copy file: %w", err)
	}
	return nil
}
