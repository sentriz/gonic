// author: spijet (https://github.com/spijet/)
// author: sentriz (https://github.com/sentriz/)

package transcode

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/google/shlex"
)

type Transcoder interface {
	Transcode(ctx context.Context, profile Profile, in string, out io.Writer) error
}

//nolint:gochecknoglobals
var UserProfiles = map[string]Profile{
	"mp3":          MP3,
	"mp3_320":      MP3320,
	"mp3_rg":       MP3RG,
	"opus_car":     OpusRGLoud,
	"opus":         Opus,
	"opus_rg":      OpusRG,
	"opus_128_car": Opus128RGLoud,
	"opus_128":     Opus128,
	"opus_128_rg":  Opus128RG,
	"opus_192":     Opus192,
}

// Store as simple strings, since we may let the user provide their own profiles soon
//
//nolint:gochecknoglobals
var (
	MP3    = NewProfile("audio/mpeg", "mp3", 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libmp3lame -f mp3 -`)
	MP3320 = NewProfile("audio/mpeg", "mp3", 320, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libmp3lame -f mp3 -`)
	MP3RG  = NewProfile("audio/mpeg", "mp3", 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libmp3lame -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f mp3 -`)

	Opus       = NewProfile("audio/ogg", "opus", 96, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr on -f opus -`)
	OpusRG     = NewProfile("audio/ogg", "opus", 96, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr on -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)
	OpusRGLoud = NewProfile("audio/ogg", "opus", 96, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr on -af "aresample=96000:resampler=soxr, volume=replaygain=track:replaygain_preamp=15dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)

	Opus128       = NewProfile("audio/ogg", "opus", 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr on -f opus -`)
	Opus128RG     = NewProfile("audio/ogg", "opus", 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr on -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)
	Opus128RGLoud = NewProfile("audio/ogg", "opus", 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr on -af "aresample=96000:resampler=soxr, volume=replaygain=track:replaygain_preamp=15dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)

	Opus192 = NewProfile("audio/ogg", "opus", 192, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr on -f opus -`)

	PCM16le = NewProfile("audio/wav", "wav", 0, `ffmpeg -v 0 -i <file> -ss <seek> -c:a pcm_s16le -ac 2 -ar 48000 -f s16le -`)
)

type BitRate uint // kilobits/s

type Profile struct {
	bitrate BitRate // the default bitrate, but the user can request a different one
	seek    time.Duration
	mime    string
	suffix  string
	exec    string
}

func (p *Profile) BitRate() BitRate    { return p.bitrate }
func (p *Profile) Seek() time.Duration { return p.seek }
func (p *Profile) Suffix() string      { return p.suffix }
func (p *Profile) MIME() string        { return p.mime }

func NewProfile(mime string, suffix string, bitrate BitRate, exec string) Profile {
	return Profile{mime: mime, suffix: suffix, bitrate: bitrate, exec: exec}
}

func WithBitrate(p Profile, bitRate BitRate) Profile {
	p.bitrate = bitRate
	return p
}

func WithSeek(p Profile, seek time.Duration) Profile {
	p.seek = seek
	return p
}

var ErrNoProfileParts = fmt.Errorf("not enough profile parts")

func parseProfile(profile Profile, in string) (string, []string, error) {
	parts, err := shlex.Split(profile.exec)
	if err != nil {
		return "", nil, fmt.Errorf("split command: %w", err)
	}
	if len(parts) == 0 {
		return "", nil, ErrNoProfileParts
	}
	name, err := exec.LookPath(parts[0])
	if err != nil {
		return "", nil, fmt.Errorf("find name: %w", err)
	}

	var args []string
	for _, p := range parts[1:] {
		switch p {
		case "<file>":
			args = append(args, in)
		case "<seek>":
			args = append(args, fmt.Sprintf("%dus", profile.Seek().Microseconds()))
		case "<bitrate>":
			args = append(args, fmt.Sprintf("%dk", profile.BitRate()))
		default:
			args = append(args, p)
		}
	}

	return name, args, nil
}
