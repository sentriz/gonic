// Copyright 2015, David Howden
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tag

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var id3v2Genres = [...]string{
	"Blues", "Classic Rock", "Country", "Dance", "Disco", "Funk", "Grunge",
	"Hip-Hop", "Jazz", "Metal", "New Age", "Oldies", "Other", "Pop", "R&B",
	"Rap", "Reggae", "Rock", "Techno", "Industrial", "Alternative", "Ska",
	"Death Metal", "Pranks", "Soundtrack", "Euro-Techno", "Ambient",
	"Trip-Hop", "Vocal", "Jazz+Funk", "Fusion", "Trance", "Classical",
	"Instrumental", "Acid", "House", "Game", "Sound Clip", "Gospel",
	"Noise", "AlternRock", "Bass", "Soul", "Punk", "Space", "Meditative",
	"Instrumental Pop", "Instrumental Rock", "Ethnic", "Gothic",
	"Darkwave", "Techno-Industrial", "Electronic", "Pop-Folk",
	"Eurodance", "Dream", "Southern Rock", "Comedy", "Cult", "Gangsta",
	"Top 40", "Christian Rap", "Pop/Funk", "Jungle", "Native American",
	"Cabaret", "New Wave", "Psychedelic", "Rave", "Showtunes", "Trailer",
	"Lo-Fi", "Tribal", "Acid Punk", "Acid Jazz", "Polka", "Retro",
	"Musical", "Rock & Roll", "Hard Rock", "Folk", "Folk-Rock",
	"National Folk", "Swing", "Fast Fusion", "Bebob", "Latin", "Revival",
	"Celtic", "Bluegrass", "Avantgarde", "Gothic Rock", "Progressive Rock",
	"Psychedelic Rock", "Symphonic Rock", "Slow Rock", "Big Band",
	"Chorus", "Easy Listening", "Acoustic", "Humour", "Speech", "Chanson",
	"Opera", "Chamber Music", "Sonata", "Symphony", "Booty Bass", "Primus",
	"Porn Groove", "Satire", "Slow Jam", "Club", "Tango", "Samba",
	"Folklore", "Ballad", "Power Ballad", "Rhythmic Soul", "Freestyle",
	"Duet", "Punk Rock", "Drum Solo", "A capella", "Euro-House", "Dance Hall",
	"Goa", "Drum & Bass", "Club-House", "Hardcore", "Terror", "Indie",
	"Britpop", "Negerpunk", "Polsk Punk", "Beat", "Christian Gangsta Rap",
	"Heavy Metal", "Black Metal", "Crossover", "Contemporary Christian",
	"Christian Rock ", "Merengue", "Salsa", "Thrash Metal", "Anime", "JPop",
	"Synthpop",
}

// id3v2Header is a type which represents an ID3v2 tag header.
type id3v2Header struct {
	Version           Format
	Unsynchronisation bool
	ExtendedHeader    bool
	Experimental      bool
	Size              int
}

// readID3v2Header reads the ID3v2 header from the given io.Reader.
// offset it number of bytes of header that was read
func readID3v2Header(r io.Reader) (h *id3v2Header, offset int, err error) {
	offset = 10
	b, err := readBytes(r, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("expected to read 10 bytes (ID3v2Header): %v", err)
	}

	if string(b[0:3]) != "ID3" {
		return nil, 0, fmt.Errorf("expected to read \"ID3\"")
	}

	b = b[3:]
	var vers Format
	switch uint(b[0]) {
	case 2:
		vers = ID3v2_2
	case 3:
		vers = ID3v2_3
	case 4:
		vers = ID3v2_4
	case 0, 1:
		fallthrough
	default:
		return nil, 0, fmt.Errorf("ID3 version: %v, expected: 2, 3 or 4", uint(b[0]))
	}

	// NB: We ignore b[1] (the revision) as we don't currently rely on it.
	h = &id3v2Header{
		Version:           vers,
		Unsynchronisation: getBit(b[2], 7),
		ExtendedHeader:    getBit(b[2], 6),
		Experimental:      getBit(b[2], 5),
		Size:              get7BitChunkedInt(b[3:7]),
	}

	if h.ExtendedHeader {
		switch vers {
		case ID3v2_3:
			b, err := readBytes(r, 4)
			if err != nil {
				return nil, 0, fmt.Errorf("expected to read 4 bytes (ID3v23 extended header len): %v", err)
			}
			// skip header, size is excluding len bytes
			extendedHeaderSize := getInt(b)
			_, err = readBytes(r, extendedHeaderSize)
			if err != nil {
				return nil, 0, fmt.Errorf("expected to read %d bytes (ID3v23 skip extended header): %v", extendedHeaderSize, err)
			}
			offset += extendedHeaderSize
		case ID3v2_4:
			b, err := readBytes(r, 4)
			if err != nil {
				return nil, 0, fmt.Errorf("expected to read 4 bytes (ID3v24 extended header len): %v", err)
			}
			// skip header, size is synchsafe int including len bytes
			extendedHeaderSize := get7BitChunkedInt(b) - 4
			_, err = readBytes(r, extendedHeaderSize)
			if err != nil {
				return nil, 0, fmt.Errorf("expected to read %d bytes (ID3v24 skip extended header): %v", extendedHeaderSize, err)
			}
			offset += extendedHeaderSize
		default:
			// nop, only 2.3 and 2.4 should have extended header
		}
	}

	return h, offset, nil
}

// id3v2FrameFlags is a type which represents the flags which can be set on an ID3v2 frame.
type id3v2FrameFlags struct {
	// Message (ID3 2.3.0 and 2.4.0)
	TagAlterPreservation  bool
	FileAlterPreservation bool
	ReadOnly              bool

	// Format (ID3 2.3.0 and 2.4.0)
	Compression   bool
	Encryption    bool
	GroupIdentity bool
	// ID3 2.4.0 only (see http://id3.org/id3v2.4.0-structure sec 4.1)
	Unsynchronisation   bool
	DataLengthIndicator bool
}

func readID3v23FrameFlags(r io.Reader) (*id3v2FrameFlags, error) {
	b, err := readBytes(r, 2)
	if err != nil {
		return nil, err
	}

	msg := b[0]
	fmt := b[1]

	return &id3v2FrameFlags{
		TagAlterPreservation:  getBit(msg, 7),
		FileAlterPreservation: getBit(msg, 6),
		ReadOnly:              getBit(msg, 5),
		Compression:           getBit(fmt, 7),
		Encryption:            getBit(fmt, 6),
		GroupIdentity:         getBit(fmt, 5),
	}, nil
}

func readID3v24FrameFlags(r io.Reader) (*id3v2FrameFlags, error) {
	b, err := readBytes(r, 2)
	if err != nil {
		return nil, err
	}

	msg := b[0]
	fmt := b[1]

	return &id3v2FrameFlags{
		TagAlterPreservation:  getBit(msg, 6),
		FileAlterPreservation: getBit(msg, 5),
		ReadOnly:              getBit(msg, 4),
		GroupIdentity:         getBit(fmt, 6),
		Compression:           getBit(fmt, 3),
		Encryption:            getBit(fmt, 2),
		Unsynchronisation:     getBit(fmt, 1),
		DataLengthIndicator:   getBit(fmt, 0),
	}, nil

}

func readID3v2_2FrameHeader(r io.Reader) (name string, size int, headerSize int, err error) {
	name, err = readString(r, 3)
	if err != nil {
		return
	}
	size, err = readInt(r, 3)
	if err != nil {
		return
	}
	headerSize = 6
	return
}

func readID3v2_3FrameHeader(r io.Reader) (name string, size int, headerSize int, err error) {
	name, err = readString(r, 4)
	if err != nil {
		return
	}
	size, err = readInt(r, 4)
	if err != nil {
		return
	}
	headerSize = 8
	return
}

func readID3v2_4FrameHeader(r io.Reader) (name string, size int, headerSize int, err error) {
	name, err = readString(r, 4)
	if err != nil {
		return
	}
	size, err = read7BitChunkedInt(r, 4)
	if err != nil {
		return
	}
	headerSize = 8
	return
}

// readID3v2Frames reads ID3v2 frames from the given reader using the ID3v2Header.
func readID3v2Frames(r io.Reader, offset int, h *id3v2Header) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for offset < h.Size {
		var err error
		var name string
		var size, headerSize int
		var flags *id3v2FrameFlags

		switch h.Version {
		case ID3v2_2:
			name, size, headerSize, err = readID3v2_2FrameHeader(r)

		case ID3v2_3:
			name, size, headerSize, err = readID3v2_3FrameHeader(r)
			if err != nil {
				return nil, err
			}
			flags, err = readID3v23FrameFlags(r)
			headerSize += 2

		case ID3v2_4:
			name, size, headerSize, err = readID3v2_4FrameHeader(r)
			if err != nil {
				return nil, err
			}
			flags, err = readID3v24FrameFlags(r)
			headerSize += 2
		}

		if err != nil {
			return nil, err
		}

		// FIXME: Do we still need this?
		// if size=0, we certainly are in a padding zone. ignore the rest of
		// the tags
		if size == 0 {
			break
		}

		offset += headerSize + size

		// Avoid corrupted padding (see http://id3.org/Compliance%20Issues).
		if !validID3Frame(h.Version, name) && offset > h.Size {
			break
		}

		if flags != nil {
			if flags.Compression {
				_, err = read7BitChunkedInt(r, 4) // read 4
				if err != nil {
					return nil, err
				}
				size -= 4
			}

			if flags.Encryption {
				_, err = readBytes(r, 1) // read 1 byte of encryption method
				if err != nil {
					return nil, err
				}
				size -= 1
			}
		}

		b, err := readBytes(r, size)
		if err != nil {
			return nil, err
		}

		// There can be multiple tag with the same name. Append a number to the
		// name if there is more than one.
		rawName := name
		if _, ok := result[rawName]; ok {
			for i := 0; ok; i++ {
				rawName = name + "_" + strconv.Itoa(i)
				_, ok = result[rawName]
			}
		}

		switch {
		case name == "TXXX" || name == "TXX":
			t, err := readTextWithDescrFrame(b, false, true) // no lang, but enc
			if err != nil {
				return nil, err
			}
			result[rawName] = t

		case name[0] == 'T':
			txt, err := readTFrame(b)
			if err != nil {
				return nil, err
			}
			result[rawName] = txt

		case name == "UFID" || name == "UFI":
			t, err := readUFID(b)
			if err != nil {
				return nil, err
			}
			result[rawName] = t

		case name == "WXXX" || name == "WXX":
			t, err := readTextWithDescrFrame(b, false, false) // no lang, no enc
			if err != nil {
				return nil, err
			}
			result[rawName] = t

		case name[0] == 'W':
			txt, err := readWFrame(b)
			if err != nil {
				return nil, err
			}
			result[rawName] = txt

		case name == "COMM" || name == "COM" || name == "USLT" || name == "ULT":
			t, err := readTextWithDescrFrame(b, true, true) // both lang and enc
			if err != nil {
				return nil, err
			}
			result[rawName] = t

		case name == "APIC":
			p, err := readAPICFrame(b)
			if err != nil {
				return nil, err
			}
			result[rawName] = p

		case name == "PIC":
			p, err := readPICFrame(b)
			if err != nil {
				return nil, err
			}
			result[rawName] = p

		default:
			result[rawName] = b
		}
	}
	return result, nil
}

type unsynchroniser struct {
	io.Reader
	ff bool
}

// filter io.Reader which skip the Unsynchronisation bytes
func (r *unsynchroniser) Read(p []byte) (int, error) {
	b := make([]byte, 1)
	i := 0
	for i < len(p) {
		if n, err := r.Reader.Read(b); err != nil || n == 0 {
			return i, err
		}
		if r.ff && b[0] == 0x00 {
			r.ff = false
			continue
		}
		p[i] = b[0]
		i++
		r.ff = (b[0] == 0xFF)
	}
	return i, nil
}

// ReadID3v2Tags parses ID3v2.{2,3,4} tags from the io.ReadSeeker into a Metadata, returning
// non-nil error on failure.
func ReadID3v2Tags(r io.ReadSeeker) (Metadata, error) {
	h, offset, err := readID3v2Header(r)
	if err != nil {
		return nil, err
	}

	var ur io.Reader = r
	if h.Unsynchronisation {
		ur = &unsynchroniser{Reader: r}
	}

	f, err := readID3v2Frames(ur, offset, h)
	if err != nil {
		return nil, err
	}
	return metadataID3v2{header: h, frames: f}, nil
}

var id3v2genreRe = regexp.MustCompile(`(.*[^(]|.* |^)\(([0-9]+)\) *(.*)$`)

//  id3v2genre parse a id3v2 genre tag and expand the numeric genres
func id3v2genre(genre string) string {
	c := true
	for c {
		orig := genre
		if match := id3v2genreRe.FindStringSubmatch(genre); len(match) > 0 {
			if genreID, err := strconv.Atoi(match[2]); err == nil {
				if genreID < len(id3v2Genres) {
					genre = id3v2Genres[genreID]
					if match[1] != "" {
						genre = strings.TrimSpace(match[1]) + " " + genre
					}
					if match[3] != "" {
						genre = genre + " " + match[3]
					}
				}
			}
		}
		c = (orig != genre)
	}
	return strings.Replace(genre, "((", "(", -1)
}
