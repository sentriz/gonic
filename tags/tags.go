package tags

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

var (
	titleFields = []string{
		"TITLE",
		"title",
	}
	albumFields = []string{
		"ALBUM",
		"album",
	}
	artistFields = []string{
		"ARTIST",
		"artist",
	}
	albumArtistFields = []string{
		"ALBUM ARTIST",
		"ALBUMARTIST",
		"ALBUM_ARTIST",
		"album artist",
		"album_artist",
		"albumartist",
	}
	composerFields = []string{
		"COMPOSER",
		"composer",
	}
	genreFields = []string{
		"GENRE",
		"genre",
	}
	yearFields = []string{
		"YEAR",
		"year",
	}
	trackFields = []string{
		"TRACK",
		"track",
	}
	totaltrackFields = []string{
		"TOTALTRACKS",
		"TRACKC",
		"TRACKTOTAL",
		"totaltracks",
		"trackc",
		"tracktotal",
	}
	discFields = []string{
		"DISC",
		"disc",
	}
	totaldiscFields = []string{
		"DISCC",
		"DISCTOTAL",
		"TOTALDISCS",
		"discc",
		"disctotal",
		"totaldiscs",
	}
	lyricsFields = []string{
		"LYRICS",
		"lyrics",
	}
	commentFields = []string{
		"COMMENT",
		"COMMENTS",
		"comment",
		"comments",
	}
)

func firstExisting(keys *[]string, map_ *map[string]string) string {
	for _, field := range *keys {
		v, ok := (*map_)[field]
		if !ok {
			continue
		}
		return v
	}
	return ""
}

type Metadata interface {
	Title() string
	Album() string
	Artist() string
	AlbumArtist() string
	Composer() string
	Genre() string
	Year() int
	Track() int
	TotalTracks() int
	Disc() int
	TotalDiscs() int
	Lyrics() string
	Comment() string
	Length() float64
	Format() string
	Bitrate() int
}

type Track struct {
	format *probeFormat
}

func (t *Track) Title() string {
	return firstExisting(&titleFields, &t.format.Tags)
}

func (t *Track) Album() string {
	return firstExisting(&albumFields, &t.format.Tags)
}

func (t *Track) Artist() string {
	return firstExisting(&artistFields, &t.format.Tags)
}

func (t *Track) AlbumArtist() string {
	return firstExisting(&albumArtistFields, &t.format.Tags)
}

func (t *Track) Composer() string {
	return firstExisting(&composerFields, &t.format.Tags)
}

func (t *Track) Genre() string {
	return firstExisting(&genreFields, &t.format.Tags)
}

func (t *Track) Year() int {
	i, _ := strconv.Atoi(firstExisting(&yearFields, &t.format.Tags))
	return i
}

func (t *Track) Track() int {
	i, _ := strconv.Atoi(firstExisting(&trackFields, &t.format.Tags))
	return i
}

func (t *Track) TotalTracks() int {
	i, _ := strconv.Atoi(firstExisting(&totaltrackFields, &t.format.Tags))
	return i
}

func (t *Track) Disc() int {
	i, _ := strconv.Atoi(firstExisting(&discFields, &t.format.Tags))
	return i
}

func (t *Track) TotalDiscs() int {
	i, _ := strconv.Atoi(firstExisting(&totaldiscFields, &t.format.Tags))
	return i
}

func (t *Track) Lyrics() string {
	return firstExisting(&lyricsFields, &t.format.Tags)
}

func (t *Track) Comment() string {
	return firstExisting(&commentFields, &t.format.Tags)
}

func (t *Track) Format() string {
	return t.format.FormatName
}

func (t *Track) Length() float64 {
	return t.format.Duration
}

func (t *Track) Bitrate() int {
	return t.format.Bitrate
}

func Read(filename string) (Metadata, error) {
	command := exec.Command(
		"ffprobe",
		"-print_format", "json",
		"-show_format",
		filename,
	)
	probe, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("when running ffprobe with `%s`: %v\n",
			filename, err)
	}
	var data probeData
	err = json.Unmarshal(probe, &data)
	if err != nil {
		return nil, fmt.Errorf("when unmarshalling: %v\n", err)
	}
	track := Track{
		format: data.Format,
	}
	return &track, nil
}
