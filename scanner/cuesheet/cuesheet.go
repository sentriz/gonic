package cuesheet

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	delims           = "\t\n\r "
	eol              = "\n"
	framesPerSecond  = 75
	remGenre         = "GENRE"
	remComment       = "COMMENT"
	remDate          = "DATE"
	remDiskID        = "DISCID"
	remRGAlbumGain   = "REPLAYGAIN_ALBUM_GAIN"
	remRGAlbumPeak   = "REPLAYGAIN_ALBUM_PEAK"
	remRGTrackGain   = "REPLAYGAIN_TRACK_GAIN"
	remRGTrackPeak   = "REPLAYGAIN_TRACK_PEAK"
	remMusicBrainzID = "MUSICBRAINZID"
	remDiscNumber    = "DISCNUMBER"
	remTotalDiscs    = "TOTALDISCS"
)

// Frame represent track frame
type Frame uint64

// Flags for track
type Flags int

// RemData data from REM lines
type RemData map[string]string

const (
	// None - no flags
	None   Flags = iota
	Dcp          = 1 << iota
	FourCh       = 1 << iota
	Pre          = 1 << iota
	Scms         = 1 << iota
)

var (
	ErrorFrameFormat = fmt.Errorf("invalid frame format")
)

// TrackIndex data
type TrackIndex struct {
	Number uint
	Frame  Frame
}

func frameFromString(str string) (Frame, error) {
	v := strings.Split(str, ":")
	if len(str) < 8 {
		return 0, ErrorFrameFormat
	}
	if len(v) == 3 {
		mm, _ := strconv.ParseUint(v[0], 10, 32)
		ss, _ := strconv.ParseUint(v[1], 10, 32)
		if ss > 59 {
			return 0, ErrorFrameFormat
		}
		ff, _ := strconv.ParseUint(v[2], 10, 32)
		if ff >= framesPerSecond {
			return 0, ErrorFrameFormat
		}
		return Frame((mm*60+ss)*framesPerSecond + ff), nil
	}
	return 0, ErrorFrameFormat
}

func (frame Frame) Duration() time.Duration {
	seconds := time.Second * time.Duration(frame/framesPerSecond)
	milliseconds := time.Millisecond * time.Duration(float64(frame%framesPerSecond)/0.075)
	return seconds + milliseconds
}

func (frame Frame) String() string {
	seconds := frame / framesPerSecond
	minutes := seconds / 60
	seconds %= 60
	ff := frame % framesPerSecond
	return fmt.Sprintf("%.2d:%.2d:%.2d", minutes, seconds, ff)
}

// Track instance
type Track struct {
	Rem           RemData
	TrackNumber   uint
	TrackDataType string
	Flags         Flags
	ISRC          string
	Title         string
	Performer     string
	SongWriter    string
	PreGap        Frame
	PostGap       Frame
	Index         []TrackIndex
}

// File instance
type File struct {
	FileName string
	FileType string
	Tracks   []Track
}

// Cuesheet instance
type Cuesheet struct {
	Rem        RemData
	Catalog    string
	CdTextFile string
	Title      string
	Performer  string
	SongWriter string
	Pregap     Frame
	Postgap    Frame
	File       []File
}

func (track *Track) GetStartOffset() time.Duration {
	if len(track.Index) > 1 {
		return track.Index[1].Frame.Duration()
	}
	return track.Index[0].Frame.Duration()
}

// ReadCue loads and parses CUESHEET from reader
func ReadCue(r io.Reader) (*Cuesheet, error) {
	b := bufio.NewReader(r)
	cuesheet := &Cuesheet{}

	bom, _, err := b.ReadRune()

	if err != nil {
		return nil, err
	}

	if bom != '\uFEFF' {
		if err := b.UnreadRune(); err != nil {
			return nil, err
		}
	}

	for {
		line, err := (*b).ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		line = strings.Trim(line, delims)
		command := readString(&line)

		switch command {
		case "REM":
			if cuesheet.Rem == nil {
				cuesheet.Rem = RemData{}
			}
			cuesheet.Rem[readString(&line)] = line
		case "CATALOG":
			cuesheet.Catalog = line
		case "CDTEXTFILE":
			cuesheet.CdTextFile = readString(&line)
		case "TITLE":
			cuesheet.Title = readString(&line)
		case "PERFORMER":
			cuesheet.Performer = readString(&line)
		case "SONGWRITER":
			cuesheet.SongWriter = readString(&line)
		case "PREGAP":
			cuesheet.Pregap, err = frameFromString(readString(&line))
			if err != nil {
				return nil, err
			}
		case "POSTGAP":
			cuesheet.Postgap, err = frameFromString(readString(&line))
			if err != nil {
				return nil, err
			}
		case "FILE":
			fname := readString(&line)
			ftype := readString(&line)
			tracks, err := readTracks(b)
			if err != nil {
				return nil, err
			}
			cuesheet.File = append(cuesheet.File, File{fname, ftype, *tracks})
		default:
		}
	}

	return cuesheet, nil
}

// WriteCue writes CUESHEET to writer
//
//gocyclo:ignore
func WriteCue(w io.Writer, cuesheet *Cuesheet) error {
	ws := bufio.NewWriter(w)
	for k := range cuesheet.Rem {
		_, err := ws.WriteString("REM " + k + " " + cuesheet.Rem[k] + eol)
		if err != nil {
			return err
		}
	}

	if len(cuesheet.Catalog) > 0 {
		_, err := ws.WriteString("CATALOG " + cuesheet.Catalog + eol)
		if err != nil {
			return err
		}
	}

	if len(cuesheet.CdTextFile) > 0 {
		_, err := ws.WriteString("CDTEXTFILE " + formatString(cuesheet.CdTextFile) + eol)
		if err != nil {
			return err
		}
	}

	if len(cuesheet.Title) > 0 {
		_, err := ws.WriteString("TITLE " + formatString(cuesheet.Title) + eol)
		if err != nil {
			return err
		}
	}

	if len(cuesheet.Performer) > 0 {
		_, err := ws.WriteString("PERFORMER " + formatString(cuesheet.Performer) + eol)
		if err != nil {
			return err
		}
	}

	if len(cuesheet.SongWriter) > 0 {
		_, err := ws.WriteString("SONGWRITER " + formatString(cuesheet.SongWriter) + eol)
		if err != nil {
			return err
		}
	}

	if cuesheet.Pregap > 0 {
		_, err := ws.WriteString("PREGAP " + cuesheet.Pregap.String() + eol)
		if err != nil {
			return err
		}
	}

	if cuesheet.Postgap > 0 {
		_, err := ws.WriteString("POSTGAP " + cuesheet.Postgap.String() + eol)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(cuesheet.File); i++ {
		file := &cuesheet.File[i]
		_, err := ws.WriteString("FILE " + formatString(file.FileName) +
			" " + file.FileType + eol)
		if err != nil {
			return err
		}

		for i := 0; i < len(file.Tracks); i++ {
			track := &file.Tracks[i]

			_, err := ws.WriteString("  TRACK " + formatTrackNumber(track.TrackNumber) +
				" " + track.TrackDataType + eol)
			if err != nil {
				return err
			}

			if track.Flags != None {
				_, err := ws.WriteString("    FLAGS")
				if err != nil {
					return err
				}
				if (track.Flags & Dcp) != 0 {
					_, err := ws.WriteString(" DCP")
					if err != nil {
						return err
					}
				}
				if (track.Flags & FourCh) != 0 {
					_, err := ws.WriteString(" 4CH")
					if err != nil {
						return err
					}
				}
				if (track.Flags & Pre) != 0 {
					_, err := ws.WriteString(" PRE")
					if err != nil {
						return err
					}
				}
				if (track.Flags & Scms) != 0 {
					_, err := ws.WriteString(" SCMS")
					if err != nil {
						return err
					}
				}
				if _, err := ws.WriteString(eol); err != nil {
					return err
				}
			}

			if len(track.ISRC) > 0 {
				_, err := ws.WriteString("    ISRC " + track.ISRC + eol)
				if err != nil {
					return err
				}
			}

			if len(track.Title) > 0 {
				_, err := ws.WriteString("    TITLE " + formatString(track.Title) + eol)
				if err != nil {
					return err
				}
			}

			if len(track.Performer) > 0 {
				_, err := ws.WriteString("    PERFORMER " + formatString(track.Performer) + eol)
				if err != nil {
					return err
				}
			}

			if len(track.SongWriter) > 0 {
				_, err := ws.WriteString("    SONGWRITER " + formatString(track.SongWriter) + eol)
				if err != nil {
					return err
				}
			}

			if track.PreGap > 0 {
				_, err := ws.WriteString("    PREGAP " + track.PreGap.String() + eol)
				if err != nil {
					return err
				}
			}

			if track.PostGap > 0 {
				_, err := ws.WriteString("    POSTGAP " + track.PostGap.String() + eol)
				if err != nil {
					return err
				}
			}

			if track.Rem != nil {
				for k := range track.Rem {
					_, err := ws.WriteString("    REM " + k + " " + track.Rem[k] + eol)
					if err != nil {
						return err
					}
				}
			}

			for i := 0; i < len(track.Index); i++ {
				index := &track.Index[i]
				_, err := ws.WriteString("    INDEX " + formatTrackNumber(index.Number) +
					" " + index.Frame.String() + eol)
				if err != nil {
					return err
				}
			}
		}
	}

	return ws.Flush()
}

func readString(s *string) string {
	*s = strings.TrimLeft(*s, delims)

	if len(*s) > 0 && isQuoted(*s) {
		v := unquote(*s)
		*s = (*s)[len(v)+2:]
		return v
	}
	for i := 0; i < len(*s); i++ {
		if (*s)[i] == ' ' {
			v := (*s)[0:i]
			*s = (*s)[i+1:]
			return v
		}
	}
	v := *s
	*s = ""
	return v
}

func readUint(s *string) uint {
	v := readString(s)
	if n, err := strconv.ParseUint(v, 10, 32); err == nil {
		return uint(n)
	}
	return 0
}

func formatString(s string) string {
	return quote(s, '"')
}

func formatTrackNumber(n uint) string {
	return leftPad(strconv.FormatUint(uint64(n), 10), "0", 2)
}

func isQuoted(s string) bool {
	if len(s) == 0 {
		return false
	}
	return s[0] == '"' || s[0] == '\''
}

func quote(s string, quote byte) string {
	buf := make([]byte, 0, 3*len(s)/2)
	buf = append(buf, quote)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == quote || c == '\\' {
			buf = append(buf, '\\')
			buf = append(buf, c)
		} else {
			buf = append(buf, c)
		}
	}
	buf = append(buf, quote)
	return string(buf)
}

func unquote(s string) string {
	quote := s[0]
	i := 1
	for ; i < len(s); i++ {
		if s[i] == quote {
			break
		}
		if s[i] == '\\' {
			i++
		}
	}
	return s[1:i]
}

func readTrack(b *bufio.Reader, track *Track) error {
	for {
		before := *b
		line, err := (*b).ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		if line == "" {
			break
		}
		if !strings.HasPrefix(line, "    ") {
			*b = before
			break
		}
		line = strings.Trim(line, delims)
		command := readString(&line)

		switch command {
		case "FLAGS":
			track.Flags = None
			for len(line) > 0 {
				switch readString(&line) {
				case "DCP":
					track.Flags |= Dcp
				case "4CH":
					track.Flags |= FourCh
				case "PRE":
					track.Flags |= Pre
				case "SCMS":
					track.Flags |= Scms
				default:
				}
			}
		case "ISRC":
			track.ISRC = line
		case "TITLE":
			track.Title = unquote(line)
		case "PERFORMER":
			track.Performer = unquote(line)
		case "SONGWRITER":
			track.SongWriter = unquote(line)
		case "PREGAP":
			track.PreGap, err = frameFromString(readString(&line))
			if err != nil {
				return err
			}
		case "POSTGAP":
			track.PostGap, err = frameFromString(readString(&line))
			if err != nil {
				return err
			}
		case "INDEX":
			index := TrackIndex{}
			index.Number = readUint(&line)
			index.Frame, err = frameFromString(readString(&line))
			if err != nil {
				return err
			}
			track.Index = append(track.Index, index)
		case "REM":
			if track.Rem == nil {
				track.Rem = RemData{}
			}
			track.Rem[readString(&line)] = line
		default:
		}
	}

	return nil
}

func readTracks(b *bufio.Reader) (*[]Track, error) {
	tracks := &[]Track{}

	for {
		before := *b
		line, err := (*b).ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(line, "  ") {
			*b = before
			break
		}
		line = strings.Trim(line, delims)
		command := readString(&line)

		switch command {
		case "TRACK":
			track := Track{}
			track.TrackNumber = readUint(&line)
			track.TrackDataType = readString(&line)
			if err := readTrack(b, &track); err != nil {
				return nil, err
			}
			*tracks = append(*tracks, track)
		default:
		}
	}

	return tracks, nil
}

func leftPad(s, padStr string, overallLen int) string {
	padCountInt := 1 + ((overallLen - len(padStr)) / len(padStr))
	var retStr = strings.Repeat(padStr, padCountInt) + s
	return retStr[(len(retStr) - overallLen):]
}

func (rem *RemData) MusicBrainzID() string {
	s, ok := (*rem)[remMusicBrainzID]
	if ok {
		return s
	}
	return ""
}

func (rem *RemData) DiscNumber() int {
	s, ok := (*rem)[remDiscNumber]
	if ok {
		result, err := strconv.Atoi(s)
		if err != nil {
			return 1
		}
		return result
	}
	return 1
}

func (rem *RemData) TotalDiscs() int {
	s, ok := (*rem)[remTotalDiscs]
	if ok {
		result, err := strconv.Atoi(s)
		if err != nil {
			return 1
		}
		return result
	}
	return 1
}

// Genre returns genre from data
func (rem *RemData) Genre() string {
	s, ok := (*rem)[remGenre]
	if ok {
		return s
	}
	return ""
}

// Comment returns comment field
func (rem *RemData) Comment() string {
	s, ok := (*rem)[remComment]
	if ok {
		return s
	}
	return ""
}

// DiskID returns disk id from data
func (rem *RemData) DiskID() string {
	s, ok := (*rem)[remDiskID]
	if ok {
		return s
	}
	return ""
}

// Date returns release year
func (rem *RemData) Date() string {
	s, ok := (*rem)[remDate]
	if ok {
		return s
	}
	return ""
}

// AlbumGain returns album replay gain value
func (rem *RemData) AlbumGain() string {
	s, ok := (*rem)[remRGAlbumGain]
	if ok {
		return s
	}
	return ""
}

// AlbumPeak returns album replay gain peak value
func (rem *RemData) AlbumPeak() string {
	s, ok := (*rem)[remRGAlbumPeak]
	if ok {
		return s
	}
	return ""
}

// TrackGain returns track replay gain value
func (rem *RemData) TrackGain() string {
	s, ok := (*rem)[remRGTrackGain]
	if ok {
		return s
	}
	return ""
}

// TrackPeak returns track replay gain peak value
func (rem *RemData) TrackPeak() string {
	s, ok := (*rem)[remRGTrackPeak]
	if ok {
		return s
	}
	return ""
}
