package iout

import "io"

type teeCloser struct {
	r io.ReadCloser
	w io.WriteCloser
}

func NewTeeCloser(r io.ReadCloser, w io.WriteCloser) io.ReadCloser {
	return &teeCloser{r, w}
}

func (t *teeCloser) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 {
		if n, err := t.w.Write(p[:n]); err != nil {
			return n, err
		}
	}
	return n, err
}

func (t *teeCloser) Close() error {
	t.r.Close()
	t.w.Close()
	return nil
}
