// Copyright 2015, David Howden
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tag

import (
	"errors"
	"io"
)

const (
	idType      int = 1
	commentType int = 3
)

// ReadOGGTags reads OGG metadata from the io.ReadSeeker, returning the resulting
// metadata in a Metadata implementation, or non-nil error if there was a problem.
// See http://www.xiph.org/vorbis/doc/Vorbis_I_spec.html
// and http://www.xiph.org/ogg/doc/framing.html for details.
func ReadOGGTags(r io.ReadSeeker) (Metadata, error) {
	oggs, err := readString(r, 4)
	if err != nil {
		return nil, err
	}
	if oggs != "OggS" {
		return nil, errors.New("expected 'OggS'")
	}

	// Skip 22 bytes of Page header to read page_segments length byte at position 26
	// See http://www.xiph.org/ogg/doc/framing.html
	_, err = r.Seek(22, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	nS, err := readInt(r, 1)
	if err != nil {
		return nil, err
	}

	// Seek and discard the segments
	_, err = r.Seek(int64(nS), io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	// First packet type is identification, type 1
	t, err := readInt(r, 1)
	if err != nil {
		return nil, err
	}
	if t != idType {
		return nil, errors.New("expected 'vorbis' identification type 1")
	}

	// Seek and discard 29 bytes from common and identification header
	// See http://www.xiph.org/vorbis/doc/Vorbis_I_spec.html#x1-610004.2
	_, err = r.Seek(29, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	// Beginning of a new page. Comment packet is on a separate page
	// See http://www.xiph.org/vorbis/doc/Vorbis_I_spec.html#x1-132000A.2
	oggs, err = readString(r, 4)
	if err != nil {
		return nil, err
	}
	if oggs != "OggS" {
		return nil, errors.New("expected 'OggS'")
	}

	// Skip page 2 header, same as line 30
	_, err = r.Seek(22, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	nS, err = readInt(r, 1)
	if err != nil {
		return nil, err
	}

	_, err = r.Seek(int64(nS), io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	// Packet type is comment, type 3
	t, err = readInt(r, 1)
	if err != nil {
		return nil, err
	}
	if t != commentType {
		return nil, errors.New("expected 'vorbis' comment type 3")
	}

	// Seek and discard 6 bytes from common header
	_, err = r.Seek(6, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	m := &metadataOGG{
		newMetadataVorbis(),
	}

	err = m.readVorbisComment(r)
	return m, err
}

type metadataOGG struct {
	*metadataVorbis
}

func (m *metadataOGG) FileType() FileType {
	return OGG
}
