package scanner_test

import (
	"fmt"
	"testing"

	"go.senan.xyz/gonic/mockfs"
)

func BenchmarkScanIncremental(b *testing.B) {
	m := mockfs.New(b)
	for i := range 5 {
		m.AddItemsPrefix(fmt.Sprintf("t-%d", i))
	}
	m.ScanAndClean()

	for b.Loop() {
		m.ScanAndClean()
	}
}

func BenchmarkScanFull(b *testing.B) {
	for b.Loop() {
		m := mockfs.New(b)
		for i := range 5 {
			m.AddItemsPrefix(fmt.Sprintf("t-%d", i))
		}
		b.StartTimer()
		m.ScanAndClean()
		b.StopTimer()
	}
}
