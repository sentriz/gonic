package iout

import "io"

type nullReader struct{}

func NewNullReader() io.Reader {
	return &nullReader{}
}

func (*nullReader) Read(p []byte) (n int, err error) {
	for b := range p {
		p[b] = 0
	}
	return len(p), nil
}

var _ io.Reader = (*nullReader)(nil)
