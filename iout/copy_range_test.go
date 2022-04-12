package iout_test

import (
	"bytes"
	"testing"

	"github.com/matryer/is"
	"go.senan.xyz/gonic/iout"
)

func TestCopyRange(t *testing.T) {
	t.Parallel()
	is := is.New(t)

	realLength := 50
	cr := func(start, length int64) []byte {
		is.Helper()
		var data []byte
		for i := 0; i < realLength; i++ {
			data = append(data, byte(i%10))
		}
		var buff bytes.Buffer
		is.NoErr(iout.CopyRange(&buff, bytes.NewReader(data), start, length))
		return buff.Bytes()
	}

	// range
	is.Equal(len(cr(0, 50)), 50)
	is.Equal(len(cr(10, 10)), 10)
	is.Equal(cr(10, 10)[0], byte(0))
	is.Equal(cr(10, 10)[5], byte(5))
	is.Equal(cr(25, 35)[0], byte(5))
	is.Equal(cr(25, 35)[5], byte(0))

	// 0 padding
	is.Equal(len(cr(0, 5000)), 5000)
	is.Equal(cr(0, 5000)[50:], make([]byte, 5000-50))

	// no bound
	is.Equal(len(cr(0, 0)), 50)
	is.Equal(len(cr(50, 0)), 0)
}
