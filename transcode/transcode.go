// author: spijet (https://github.com/spijet/)
// author: sentriz (https://github.com/sentriz/)

//nolint:gochecknoglobals
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

var UserProfiles = map[string]Profile{
	"mp3":          MP3,
	"mp3_rg":       MP3RG,
	"opus_car":     OpusRGLoud,
	"opus":         Opus,
	"opus_rg":      OpusRG,
	"opus_128_car": Opus128RGLoud,
	"opus_128":     Opus128,
	"opus_128_rg":  Opus128RG,
}

// Store as simple strings, since we may let the user provide their own profiles soon
var (
	MP3   = NewProfile("audio/mpeg", "mp3", 128, `ffmpeg -v 0 -i <file> -map 0:a:0 -vn -b:a <bitrate> -c:a libmp3lame -f mp3 -`)
	MP3RG = NewProfile("audio/mpeg", "mp3", 128, `ffmpeg -v 0 -i <file> -map 0:a:0 -vn -b:a <bitrate> -c:a libmp3lame -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f mp3 -`)

	Opus       = NewProfile("audio/ogg", "opus", 96, `ffmpeg -v 0 -i <file> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr on -f opus -`)
	OpusRG     = NewProfile("audio/ogg", "opus", 96, `ffmpeg -v 0 -i <file> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr on -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)
	OpusRGLoud = NewProfile("audio/ogg", "opus", 96, `ffmpeg -v 0 -i <file> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr on -af "aresample=96000:resampler=soxr, volume=replaygain=track:replaygain_preamp=15dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)

	Opus128       = NewProfile("audio/ogg", "opus", 128, `ffmpeg -v 0 -i <file> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr on -f opus -`)
	Opus128RG     = NewProfile("audio/ogg", "opus", 128, `ffmpeg -v 0 -i <file> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr on -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)
	Opus128RGLoud = NewProfile("audio/ogg", "opus", 128, `ffmpeg -v 0 -i <file> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr on -af "aresample=96000:resampler=soxr, volume=replaygain=track:replaygain_preamp=15dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)
)

type BitRate uint // kilobits/s

type Profile struct {
	bitrate  BitRate // the default bitrate, but the user can request a different one
	seek     time.Duration
	duration time.Duration
	mime     string
	suffix   string
	exec     string
}

func (p *Profile) BitRate() BitRate { return p.bitrate }
func (p *Profile) Suffix() string   { return p.suffix }
func (p *Profile) MIME() string     { return p.mime }

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

func WithInterval(p Profile, start, duration time.Duration) Profile {
	p.seek = start
	p.duration = duration
	return p
}

var ErrNoProfileParts = fmt.Errorf("not enough profile parts")

func formatDuration(dur time.Duration) string {
	z := time.Unix(0, 0).UTC()
	return z.Add(dur).Format("15:04:05.000")
}

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
			if profile.seek > time.Duration(0) {
				args = append(args, "-ss", formatDuration(profile.seek))
			}
			if profile.duration > time.Duration(0) {
				args = append(args, "-t", formatDuration(profile.duration))
			}
		case "<bitrate>":
			args = append(args, fmt.Sprintf("%dk", profile.BitRate()))
		default:
			args = append(args, p)
		}
	}

	return name, args, nil
}
