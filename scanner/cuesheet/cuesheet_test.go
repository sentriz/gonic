package cuesheet

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const cueFile = "test.cue"

func TestFrame_Duration(t *testing.T) {
	tests := []struct {
		input  Frame
		output time.Duration
	}{
		{
			input:  Frame(0),
			output: time.Duration(0),
		},
		{
			input:  Frame(1),
			output: time.Millisecond * 13,
		},
		{
			input:  Frame(74),
			output: time.Millisecond * 986,
		},
		{
			input:  Frame(75),
			output: time.Second,
		},
		{
			input:  Frame(76),
			output: time.Second + time.Millisecond*13,
		},
	}

	for _, test := range tests {
		t.Run(test.input.String(), func(t *testing.T) {
			require.Equal(t, test.output, test.input.Duration())
		})
	}
}

func TestFrameFromStringToString(t *testing.T) {
	tests := []struct {
		input  string
		output Frame
		err    error
	}{
		{
			input:  "14:15:70",
			output: Frame(0xfac3),
		},
		{
			input: "0:0:0",
			err:   fmt.Errorf("invalid frame format"),
		},
		{
			input:  "00:00:00",
			output: Frame(0),
		},
		{
			input:  "00:00:10",
			output: Frame(10),
		},
		{
			input:  "00:01:74",
			output: Frame(0x95),
		},
		{
			input:  "155:01:01",
			output: Frame(0xaa4e8),
		},
		{
			input:  "02:59:01",
			output: Frame(0xaa4e8),
		},
		{
			input:  "01:01:01",
			output: Frame(0x11e0),
		},
		{
			input:  "00:01:75",
			output: Frame(0),
			err:    fmt.Errorf("invalid frame format"),
		},
		{
			input:  "invalid",
			output: Frame(0),
			err:    fmt.Errorf("invalid frame format"),
		},
		{
			input:  "001:60:73",
			output: Frame(0),
			err:    fmt.Errorf("invalid frame format"),
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result, err := FrameFromString(test.input)
			if test.err != nil {
				require.EqualError(t, err, test.err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, test.output, result)
				require.Equal(t, test.input, result.String())
			}
		})
	}
}

func TestCuesheet(t *testing.T) {
	origCue, err := os.ReadFile(filepath.Join("testdata", cueFile))
	assert.NoError(t, err)

	reader := bytes.NewReader(origCue)
	src, err := ReadCue(reader)

	assert.Equal(t, 6, len(src.Rem), "Read all file REMs")

	assert.NoError(t, err)

	var w bytes.Buffer

	assert.NoError(t, WriteCue(&w, src))

	result := w.Bytes()

	reader.Reset(result)
	target, err := ReadCue(reader)
	assert.NoError(t, err)

	assert.Equal(t, src, target)
}
