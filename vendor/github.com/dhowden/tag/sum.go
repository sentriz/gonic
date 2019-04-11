package tag

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
)

// Sum creates a checksum of the audio file data provided by the io.ReadSeeker which is metadata
// (ID3, MP4) invariant.
func Sum(r io.ReadSeeker) (string, error) {
	b, err := readBytes(r, 11)
	if err != nil {
		return "", err
	}

	_, err = r.Seek(-11, io.SeekCurrent)
	if err != nil {
		return "", fmt.Errorf("could not seek back to original position: %v", err)
	}

	switch {
	case string(b[0:4]) == "fLaC":
		return SumFLAC(r)

	case string(b[4:11]) == "ftypM4A":
		return SumAtoms(r)

	case string(b[0:3]) == "ID3":
		return SumID3v2(r)
	}

	h, err := SumID3v1(r)
	if err != nil {
		if err == ErrNotID3v1 {
			return SumAll(r)
		}
		return "", err
	}
	return h, nil
}

// SumAll returns a checksum of the content from the reader (until EOF).
func SumAll(r io.ReadSeeker) (string, error) {
	h := sha1.New()
	_, err := io.Copy(h, r)
	if err != nil {
		return "", nil
	}
	return hashSum(h), nil
}

// SumAtoms constructs a checksum of MP4 audio file data provided by the io.ReadSeeker which is
// metadata invariant.
func SumAtoms(r io.ReadSeeker) (string, error) {
	for {
		var size uint32
		err := binary.Read(r, binary.BigEndian, &size)
		if err != nil {
			if err == io.EOF {
				return "", fmt.Errorf("reached EOF before audio data")
			}
			return "", err
		}

		name, err := readString(r, 4)
		if err != nil {
			return "", err
		}

		switch name {
		case "meta":
			// next_item_id (int32)
			_, err := r.Seek(4, io.SeekCurrent)
			if err != nil {
				return "", err
			}
			fallthrough

		case "moov", "udta", "ilst":
			continue

		case "mdat": // stop when we get to the data
			h := sha1.New()
			_, err := io.CopyN(h, r, int64(size-8))
			if err != nil {
				return "", fmt.Errorf("error reading audio data: %v", err)
			}
			return hashSum(h), nil
		}

		_, err = r.Seek(int64(size-8), io.SeekCurrent)
		if err != nil {
			return "", fmt.Errorf("error reading '%v' tag: %v", name, err)
		}
	}
}

func sizeToEndOffset(r io.ReadSeeker, offset int64) (int64, error) {
	n, err := r.Seek(-128, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("error seeking end offset (%d bytes): %v", offset, err)
	}

	_, err = r.Seek(-n, io.SeekCurrent)
	if err != nil {
		return 0, fmt.Errorf("error seeking back to original position: %v", err)
	}
	return n, nil
}

// SumID3v1 constructs a checksum of MP3 audio file data (assumed to have ID3v1 tags) provided
// by the io.ReadSeeker which is metadata invariant.
func SumID3v1(r io.ReadSeeker) (string, error) {
	n, err := sizeToEndOffset(r, 128)
	if err != nil {
		return "", fmt.Errorf("error determining read size to ID3v1 header: %v", err)
	}

	// TODO: improve this check???
	if n <= 0 {
		return "", fmt.Errorf("file size must be greater than 128 bytes (ID3v1 header size) for MP3")
	}

	h := sha1.New()
	_, err = io.CopyN(h, r, n)
	if err != nil {
		return "", fmt.Errorf("error reading %v bytes: %v", n, err)
	}
	return hashSum(h), nil
}

// SumID3v2 constructs a checksum of MP3 audio file data (assumed to have ID3v2 tags) provided by the
// io.ReadSeeker which is metadata invariant.
func SumID3v2(r io.ReadSeeker) (string, error) {
	header, _, err := readID3v2Header(r)
	if err != nil {
		return "", fmt.Errorf("error reading ID3v2 header: %v", err)
	}

	_, err = r.Seek(int64(header.Size), io.SeekCurrent)
	if err != nil {
		return "", fmt.Errorf("error seeking to end of ID3V2 header: %v", err)
	}

	n, err := sizeToEndOffset(r, 128)
	if err != nil {
		return "", fmt.Errorf("error determining read size to ID3v1 header: %v", err)
	}

	// TODO: remove this check?????
	if n < 0 {
		return "", fmt.Errorf("file size must be greater than 128 bytes for MP3: %v bytes", n)
	}

	h := sha1.New()
	_, err = io.CopyN(h, r, n)
	if err != nil {
		return "", fmt.Errorf("error reading %v bytes: %v", n, err)
	}
	return hashSum(h), nil
}

// SumFLAC costructs a checksum of the FLAC audio file data provided by the io.ReadSeeker (ignores
// metadata fields).
func SumFLAC(r io.ReadSeeker) (string, error) {
	flac, err := readString(r, 4)
	if err != nil {
		return "", err
	}
	if flac != "fLaC" {
		return "", errors.New("expected 'fLaC'")
	}

	for {
		last, err := skipFLACMetadataBlock(r)
		if err != nil {
			return "", err
		}

		if last {
			break
		}
	}

	h := sha1.New()
	_, err = io.Copy(h, r)
	if err != nil {
		return "", fmt.Errorf("error reading data bytes from FLAC: %v", err)
	}
	return hashSum(h), nil
}

func skipFLACMetadataBlock(r io.ReadSeeker) (last bool, err error) {
	blockHeader, err := readBytes(r, 1)
	if err != nil {
		return
	}

	if getBit(blockHeader[0], 7) {
		blockHeader[0] ^= (1 << 7)
		last = true
	}

	blockLen, err := readInt(r, 3)
	if err != nil {
		return
	}

	_, err = r.Seek(int64(blockLen), io.SeekCurrent)
	return
}

func hashSum(h hash.Hash) string {
	return fmt.Sprintf("%x", h.Sum([]byte{}))
}
