//go:build go1.18
// +build go1.18

package scanner_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/stretchr/testify/assert"
	"go.senan.xyz/gonic/mockfs"
)

func FuzzScanner(f *testing.F) {
	checkDelta := func(assert *assert.Assertions, m *mockfs.MockFS, expSeen, expNew int) {
		ctx := m.ScanAndClean()
		assert.Equal(ctx.SeenTracks(), expSeen)
		assert.Equal(ctx.SeenTracksNew(), expNew)
		assert.Equal(ctx.TracksMissing(), 0)
		assert.Equal(ctx.AlbumsMissing(), 0)
		assert.Equal(ctx.ArtistsMissing(), 0)
		assert.Equal(ctx.GenresMissing(), 0)
	}

	f.Fuzz(func(t *testing.T, data []byte, seed int64) {
		assert := assert.New(t)
		m := mockfs.New(t)

		const toAdd = 1000
		for i := 0; i < toAdd; i++ {
			path := fmt.Sprintf("artist-%d/album-%d/track-%d.flac", i/6, i/3, i)
			m.AddTrack(path)
			m.SetTags(path, func(tags *mockfs.Tags) error {
				fuzzStruct(i, data, seed, tags)
				return nil
			})
		}

		checkDelta(assert, m, toAdd, toAdd) // we added all tracks, 0 delta
		checkDelta(assert, m, toAdd, 0)     // we added 0 tracks, 0 delta
	})
}

func fuzzStruct(taken int, data []byte, seed int64, dest interface{}) {
	if len(data) == 0 {
		return
	}

	r := rand.New(rand.NewSource(seed))
	v := reflect.ValueOf(dest)
	for i := 0; i < v.Elem().NumField(); i++ {
		if r.Float64() < 0.1 {
			continue
		}

		take := int(r.Float64() * 12)
		b := make([]byte, take)
		for i := range b {
			b[i] = data[(i+taken)%len(data)]
		}
		taken += take

		switch f := v.Elem().Field(i); f.Kind() {
		case reflect.Bool:
			f.SetBool(b[0] < 128)
		case reflect.String:
			f.SetString(string(b))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			f.SetInt(int64(b[0]))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			f.SetUint(uint64(b[0]))
		case reflect.Float32, reflect.Float64:
			f.SetFloat(float64(b[0]))
		case reflect.Struct:
			fuzzStruct(taken, data, seed, f.Addr().Interface())
		}
	}
}
