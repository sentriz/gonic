package transcode

import (
	"context"
	"io"
	"os"
)

type NoneTranscoder struct{}

var _ Transcoder = (*NoneTranscoder)(nil)

func NewNoneTranscoder() *NoneTranscoder {
	return &NoneTranscoder{}
}

func (*NoneTranscoder) Transcode(ctx context.Context, _ Profile, in string) (io.ReadCloser, error) {
	return os.Open(in)
}
