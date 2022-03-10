package countrw

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
