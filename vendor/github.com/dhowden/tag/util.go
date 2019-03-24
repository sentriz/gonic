// Copyright 2015, David Howden
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tag

import (
	"encoding/binary"
	"io"
)

func getBit(b byte, n uint) bool {
	x := byte(1 << n)
	return (b & x) == x
}

func get7BitChunkedInt(b []byte) int {
	var n int
	for _, x := range b {
		n = n << 7
		n |= int(x)
	}
	return n
}

func getInt(b []byte) int {
	var n int
	for _, x := range b {
		n = n << 8
		n |= int(x)
	}
	return n
}

func getIntLittleEndian(b []byte) int {
	var n int
	for i := len(b) - 1; i >= 0; i-- {
		n = n << 8
		n |= int(b[i])
	}
	return n
}

func readBytes(r io.Reader, n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func readString(r io.Reader, n int) (string, error) {
	b, err := readBytes(r, n)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func readInt(r io.Reader, n int) (int, error) {
	b, err := readBytes(r, n)
	if err != nil {
		return 0, err
	}
	return getInt(b), nil
}

func read7BitChunkedInt(r io.Reader, n int) (int, error) {
	b, err := readBytes(r, n)
	if err != nil {
		return 0, err
	}
	return get7BitChunkedInt(b), nil
}

func readInt32LittleEndian(r io.Reader) (int, error) {
	var n int32
	err := binary.Read(r, binary.LittleEndian, &n)
	return int(n), err
}
