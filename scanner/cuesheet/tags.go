package cuesheet

import (
	"errors"
	"fmt"
	"go.senan.xyz/gonic/mime"
	"go.senan.xyz/gonic/scanner/tags"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrorInvalidCUE       = errors.New("invalid CUE")
	ErrorInvalidMediaPath = errors.New("invalid media paths")
	ErrorInputParams      = errors.New("invalid input params")
	ErrorInvalidCallback  = errors.New("invalid callback")
	ErrorUnsupportedMedia = errors.New("unsupported media")
	ErrorSkipCUE          = errors.New("skip CUE")
)

var _ tags.MetaDataProvider = (*tagsMapper)(nil)

type tagsMapper struct {
	cue          *Cuesheet
	fileIndex    int
	trackIndex   int
	mediaAbsPath []string
	mediaParsers []tags.MetaDataProvider
}

func MakeDataMapper(aCue *Cuesheet, tagsReader tags.Reader, aAbsDir string, skipWhenHasEmbedded bool, mediaPaths []string, parsers []tags.MetaDataProvider) (tags.MetaDataProvider, error) {
	if aCue == nil {
		return nil, ErrorInvalidCUE
	}
	if parsers == nil {
		if len(mediaPaths) > 0 {
			return nil, ErrorInvalidMediaPath
		}
		for _, file := range aCue.File {
			if mime.TypeByAudioExtension(filepath.Ext(file.FileName)) == "" {
				return nil, ErrorUnsupportedMedia
			}
			mediaPath := filepath.Join(aAbsDir, file.FileName)
			parser, err := tagsReader.Read(mediaPath)
			if err != nil {
				return nil, fmt.Errorf("can't read media: %w", err)
			}
			if skipWhenHasEmbedded {
				if provider, ok := parser.(tags.EmbeddedCueProvider); ok && provider.CueSheet() != "" {
					return nil, ErrorSkipCUE
				}
			}
			mediaPaths = append(mediaPaths, mediaPath)
			parsers = append(parsers, parser)
		}
	}

	if len(mediaPaths) != len(parsers) || len(aCue.File) != len(parsers) {
		return nil, ErrorInputParams
	}

	return &tagsMapper{
		cue:          aCue,
		mediaAbsPath: mediaPaths,
		mediaParsers: parsers,
	}, nil
}

func (mapper *tagsMapper) track() *Track {
	return &mapper.cue.File[mapper.fileIndex].Tracks[mapper.trackIndex]
}

type TrackCallback func(absMediaPath string, trackIndex int, trackOffset int, reader tags.MetaDataProvider) error

func (mapper *tagsMapper) ForEachTrack(callback TrackCallback) error {
	if callback == nil {
		return ErrorInvalidCallback
	}
	var file File
	for mapper.fileIndex, file = range mapper.cue.File {
		for mapper.trackIndex = range file.Tracks {
			if err := callback(mapper.mediaAbsPath[mapper.fileIndex], mapper.trackIndex, int(mapper.track().GetStartOffset().Milliseconds()), mapper); err != nil {
				return err
			}
		}
	}
	return nil
}

func (mapper *tagsMapper) Title() string {
	return mapper.track().Title
}

func (mapper *tagsMapper) CueSheet() string {
	return ""
}

func (mapper *tagsMapper) BrainzID() string {
	return mapper.track().Rem.MusicBrainzID()
}

func (mapper *tagsMapper) Artist() string {
	return tags.First(tags.UnknownArtist, mapper.track().Performer, mapper.cue.Performer)
}

func (mapper *tagsMapper) Album() string {
	return mapper.cue.Title
}

func (mapper *tagsMapper) AlbumArtist() string {
	result := ""
	for _, file := range mapper.cue.File {
		for _, track := range file.Tracks {
			artist := tags.First(track.Performer, mapper.cue.Performer)
			if artist == "" {
				continue
			}

			if result != "" && !strings.EqualFold(result, artist) {
				return "VA"
			}
			result = artist
		}
	}
	return result
}

func (mapper *tagsMapper) AlbumBrainzID() string {
	return mapper.cue.Rem.MusicBrainzID()
}

func (mapper *tagsMapper) Genre() string {
	return mapper.cue.Rem.Genre()
}

func (mapper *tagsMapper) TrackNumber() int {
	return int(mapper.track().TrackNumber)
}

func (mapper *tagsMapper) DiscNumber() int {
	return mapper.cue.Rem.DiscNumber()
}

func (mapper *tagsMapper) TotalDiscs() int {
	return mapper.cue.Rem.TotalDiscs()
}

func (mapper *tagsMapper) Length() int {
	lastTrack := len(mapper.cue.File[mapper.fileIndex].Tracks) - 1
	if lastTrack < 0 {
		return 0
	}
	var length time.Duration
	if mapper.trackIndex < lastTrack {
		nextTrack := &mapper.cue.File[mapper.fileIndex].Tracks[mapper.trackIndex+1]
		length = nextTrack.GetStartOffset() - mapper.track().GetStartOffset()
	} else {
		length = (time.Millisecond * time.Duration(mapper.mediaParsers[mapper.fileIndex].Length())) - mapper.track().GetStartOffset()
	}

	return int(length.Milliseconds())
}

func (mapper *tagsMapper) Bitrate() int {
	return mapper.mediaParsers[mapper.fileIndex].Bitrate()
}

func (mapper *tagsMapper) Year() int {
	if year, err := strconv.Atoi(mapper.cue.Rem.Date()); err == nil {
		return year
	}
	return 0
}

func (mapper *tagsMapper) SomeAlbum() string {
	return tags.First(tags.UnknownAlbum, mapper.Album())
}

func (mapper *tagsMapper) SomeArtist() string {
	return tags.First(tags.UnknownArtist, mapper.track().Performer, mapper.cue.Performer)
}

func (mapper *tagsMapper) SomeAlbumArtist() string {
	return tags.First(
		tags.UnknownArtist,
		mapper.AlbumArtist())
}

func (mapper *tagsMapper) SomeGenre() string {
	return tags.First(tags.UnknownGenre, mapper.Genre())
}

func (mapper *tagsMapper) GetMediaFiles() []string {
	var files []string
	for _, file := range mapper.mediaAbsPath {
		files = append(files, filepath.Base(file))
	}
	return files
}
