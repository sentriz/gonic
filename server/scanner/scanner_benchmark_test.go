package scanner_test

import (
	"fmt"
	"testing"

	"go.senan.xyz/gonic/server/mockfs"
)

func BenchmarkScanIncremental(b *testing.B) {
	m := mockfs.New(b)
	for i := 0; i < 5; i++ {
		m.AddItemsPrefix(fmt.Sprintf("t-%d", i))
	}
	m.ScanAndClean()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.ScanAndClean()
	}
}

func BenchmarkScanFull(b *testing.B) {
	for i := 0; i < b.N; i++ {
		m := mockfs.New(b)
		for i := 0; i < 5; i++ {
			m.AddItemsPrefix(fmt.Sprintf("t-%d", i))
		}
		b.StartTimer()
		m.ScanAndClean()
		b.StopTimer()
	}
}
