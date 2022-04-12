package iout

import (
	"io"
	"sync/atomic"
)

type CountReader struct {
	r io.Reader
	c *uint64
}

func NewCountReader(r io.Reader) *CountReader {
	return &CountReader{r: r, c: new(uint64)}
}

func (c *CountReader) Reset()        { atomic.StoreUint64(c.c, 0) }
func (c *CountReader) Count() uint64 { return atomic.LoadUint64(c.c) }

func (c *CountReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	atomic.AddUint64(c.c, uint64(n))
	return n, err
}

var _ io.Reader = (*CountReader)(nil)

type CountWriter struct {
	r io.Writer
	c *uint64
}

func NewCountWriter(r io.Writer) *CountWriter {
	return &CountWriter{r: r, c: new(uint64)}
}

func (c *CountWriter) Reset()        { atomic.StoreUint64(c.c, 0) }
func (c *CountWriter) Count() uint64 { return atomic.LoadUint64(c.c) }

func (c *CountWriter) Write(p []byte) (int, error) {
	n, err := c.r.Write(p)
	atomic.AddUint64(c.c, uint64(n))
	return n, err
}

var _ io.Writer = (*CountWriter)(nil)
