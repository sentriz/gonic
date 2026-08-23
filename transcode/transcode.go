// author: spijet (https://github.com/spijet/)
// author: sentriz (https://github.com/sentriz/)

//nolint:gochecknoglobals,goconst,gochecknoinits
package transcode

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/google/shlex"
)

type Transcoder interface {
	Transcode(ctx context.Context, profile Profile, in string, out io.Writer) error
}

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

// DefaultProfiles is the default profile per codec, used when nothing more specific was configured
var DefaultProfiles = map[CodecName]Profile{}

func init() {
	for _, p := range []Profile{MP3, Opus, FLAC} {
		DefaultProfiles[p.Codec().Name] = p
	}
}

// Store as simple strings, since we may let the user provide their own profiles soon
var (
	MP3    = NewProfile(CodecMP3, 128, `ffmpeg -v 0 -i {{ quote .File }} {{ opt "-ss" .Seek }} -map 0:a:0 -vn {{ opt "-ac" .Channels }} {{ opt "-ar" .SampleRate }} {{ if .BitRate }} -b:a {{ .BitRate }}k {{ end }} -c:a libmp3lame -f mp3 -`)
	MP3320 = NewProfile(CodecMP3, 320, `ffmpeg -v 0 -i {{ quote .File }} {{ opt "-ss" .Seek }} -map 0:a:0 -vn {{ opt "-ac" .Channels }} {{ opt "-ar" .SampleRate }} {{ if .BitRate }} -b:a {{ .BitRate }}k {{ end }} -c:a libmp3lame -f mp3 -`)
	MP3RG  = NewProfile(CodecMP3, 128, `ffmpeg -v 0 -i {{ quote .File }} {{ opt "-ss" .Seek }} -map 0:a:0 -vn {{ opt "-ac" .Channels }} {{ opt "-ar" .SampleRate }} {{ if .BitRate }} -b:a {{ .BitRate }}k {{ end }} -c:a libmp3lame -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f mp3 -`)

	Opus       = NewProfile(CodecOpus, 96, `ffmpeg -v 0 -i {{ quote .File }} {{ opt "-ss" .Seek }} -map 0:a:0 -vn {{ opt "-ac" .Channels }} {{ opt "-ar" .SampleRate }} {{ if .BitRate }} -b:a {{ .BitRate }}k {{ end }} -c:a libopus -vbr constrained -f opus -`)
	OpusRG     = NewProfile(CodecOpus, 96, `ffmpeg -v 0 -i {{ quote .File }} {{ opt "-ss" .Seek }} -map 0:a:0 -vn {{ opt "-ac" .Channels }} {{ opt "-ar" .SampleRate }} {{ if .BitRate }} -b:a {{ .BitRate }}k {{ end }} -c:a libopus -vbr constrained -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)
	OpusRGLoud = NewProfile(CodecOpus, 96, `ffmpeg -v 0 -i {{ quote .File }} {{ opt "-ss" .Seek }} -map 0:a:0 -vn {{ opt "-ac" .Channels }} {{ opt "-ar" .SampleRate }} {{ if .BitRate }} -b:a {{ .BitRate }}k {{ end }} -c:a libopus -vbr constrained -af "aresample=96000:resampler=soxr, volume=replaygain=track:replaygain_preamp=15dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)

	Opus128       = NewProfile(CodecOpus, 128, `ffmpeg -v 0 -i {{ quote .File }} {{ opt "-ss" .Seek }} -map 0:a:0 -vn {{ opt "-ac" .Channels }} {{ opt "-ar" .SampleRate }} {{ if .BitRate }} -b:a {{ .BitRate }}k {{ end }} -c:a libopus -vbr constrained -f opus -`)
	Opus128RG     = NewProfile(CodecOpus, 128, `ffmpeg -v 0 -i {{ quote .File }} {{ opt "-ss" .Seek }} -map 0:a:0 -vn {{ opt "-ac" .Channels }} {{ opt "-ar" .SampleRate }} {{ if .BitRate }} -b:a {{ .BitRate }}k {{ end }} -c:a libopus -vbr constrained -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)
	Opus128RGLoud = NewProfile(CodecOpus, 128, `ffmpeg -v 0 -i {{ quote .File }} {{ opt "-ss" .Seek }} -map 0:a:0 -vn {{ opt "-ac" .Channels }} {{ opt "-ar" .SampleRate }} {{ if .BitRate }} -b:a {{ .BitRate }}k {{ end }} -c:a libopus -vbr constrained -af "aresample=96000:resampler=soxr, volume=replaygain=track:replaygain_preamp=15dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)

	Opus192 = NewProfile(CodecOpus, 192, `ffmpeg -v 0 -i {{ quote .File }} {{ opt "-ss" .Seek }} -map 0:a:0 -vn {{ opt "-ac" .Channels }} {{ opt "-ar" .SampleRate }} {{ if .BitRate }} -b:a {{ .BitRate }}k {{ end }} -c:a libopus -vbr constrained -f opus -`)

	// lossless, so no bitrate. for resampling or bit depth conversion when a client can't play the source as-is
	FLAC = NewProfile(CodecFLAC, 0, `ffmpeg -v 0 -i {{ quote .File }} {{ opt "-ss" .Seek }} -map 0:a:0 -vn {{ opt "-ac" .Channels }} {{ opt "-ar" .SampleRate }} {{ if .BitDepth }} -sample_fmt {{ if le .BitDepth 16 }}s16{{ else }}s32{{ end }} {{ end }} -c:a flac -f flac -`)

	PCM16le = NewProfile(CodecPCM, 0, `ffmpeg -v 0 -i {{ quote .File }} {{ opt "-ss" .Seek }} -c:a pcm_s16le -ac 2 -ar 48000 -f s16le -`)
)

type CodecName string

// Codec describes an output codec and the ffmpeg encoder behind it. libmp3lame and libopus only encode
// discrete sample rates and cap their channel counts; other -ar or -ac values hard-fail
type Codec struct {
	Name        CodecName
	MIME        string
	Suffix      string
	MaxChannels int
	SampleRates []int // empty resamples freely
	BitDepths   []int // empty keeps the source's
	Lossless    bool
}

var (
	CodecMP3  = Codec{Name: "mp3", MIME: "audio/mpeg", Suffix: "mp3", MaxChannels: 2, SampleRates: []int{8000, 11025, 12000, 16000, 22050, 24000, 32000, 44100, 48000}}
	CodecOpus = Codec{Name: "opus", MIME: "audio/ogg", Suffix: "opus", MaxChannels: 8, SampleRates: []int{8000, 12000, 16000, 24000, 48000}}
	CodecFLAC = Codec{Name: "flac", MIME: "audio/flac", Suffix: "flac", MaxChannels: 8, BitDepths: []int{16, 24}, Lossless: true}
	CodecPCM  = Codec{Name: "pcm", MIME: "audio/wav", Suffix: "wav", MaxChannels: 2, Lossless: true}
)

var Codecs = map[CodecName]Codec{}

func init() {
	for _, c := range []Codec{CodecMP3, CodecOpus, CodecFLAC, CodecPCM} {
		Codecs[c.Name] = c
	}
}

func NearestSampleRate(c Codec, rate int) int {
	if len(c.SampleRates) == 0 || slices.Contains(c.SampleRates, rate) {
		return rate
	}
	var best int
	for _, s := range c.SampleRates {
		switch {
		case s >= rate && (best < rate || s < best):
			best = s
		case s < rate && best < rate && s > best:
			best = s
		}
	}
	return best
}

func NearestBitDepth(c Codec, depth int) int {
	best := 0
	for _, d := range c.BitDepths {
		if d <= depth && d > best {
			best = d
		}
	}
	return best
}

type BitRate uint // kilobits/s

type Profile struct {
	bitrate    BitRate // the default bitrate, but the user can request a different one
	seek       time.Duration
	channels   int // 0 keeps the source's channel count
	sampleRate int // 0 keeps the source's sample rate
	bitDepth   int // 0 keeps the source's bit depth
	codec      Codec
	exec       string
}

func (p Profile) BitRate() BitRate    { return p.bitrate }
func (p Profile) Seek() time.Duration { return p.seek }
func (p Profile) Codec() Codec        { return p.codec }
func (p Profile) Suffix() string      { return p.codec.Suffix }
func (p Profile) MIME() string        { return p.codec.MIME }

func NewProfile(codec Codec, bitrate BitRate, exec string) Profile {
	return Profile{codec: codec, bitrate: bitrate, exec: exec}
}

func WithBitrate(p Profile, bitRate BitRate) Profile {
	p.bitrate = bitRate
	return p
}

func WithSeek(p Profile, seek time.Duration) Profile {
	p.seek = seek
	return p
}

func WithChannels(p Profile, channels int) Profile {
	p.channels = channels
	return p
}

func WithSampleRate(p Profile, sampleRate int) Profile {
	p.sampleRate = sampleRate
	return p
}

func WithBitDepth(p Profile, bitDepth int) Profile {
	p.bitDepth = bitDepth
	return p
}

func EstimateSize(p Profile, d time.Duration) int64 {
	// headroom so container framing and tags land under the estimate, to be padded rather than clipped.
	// the overshoot is largest on short tracks (fixed header cost), peaking ~1.85% in testing, so 3% leaves margin
	const headroomPct = 3
	bytesPerSec := float64(p.BitRate()) * 1000 / 8
	return int64(d.Seconds() * bytesPerSec * (100 + headroomPct) / 100)
}

var ErrNoProfileParts = fmt.Errorf("not enough profile parts")

func parseProfile(profile Profile, in string) (string, []string, error) {
	templ, err := template.New("profile").Funcs(profileFuncs).Parse(profile.exec)
	if err != nil {
		return "", nil, fmt.Errorf("parse profile: %w", err)
	}

	var rendered strings.Builder
	if err := templ.Execute(&rendered, profileData{
		File:       in,
		Seek:       profile.seek.Seconds(),
		BitRate:    int(profile.bitrate),
		Channels:   profile.channels,
		SampleRate: profile.sampleRate,
		BitDepth:   profile.bitDepth,
	}); err != nil {
		return "", nil, fmt.Errorf("render profile: %w", err)
	}

	parts, err := shlex.Split(rendered.String())
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

	return name, parts[1:], nil
}

type profileData struct {
	File       string  // path, needs quoting
	Seek       float64 // seconds
	BitRate    int     // kilobits/s
	Channels   int
	SampleRate int
	BitDepth   int
}

var profileFuncs = template.FuncMap{
	"opt": func(flag string, value any) string {
		v := formatValue(value)
		if v == "" {
			return ""
		}
		return flag + " " + v
	},
	"quote": func(value any) string {
		return "'" + strings.ReplaceAll(formatValue(value), "'", `'\''`) + "'"
	},
}

func formatValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		if v == 0 {
			return ""
		}
		return strconv.Itoa(v)
	case float64:
		if v == 0 {
			return ""
		}
		return strconv.FormatFloat(v, 'f', -1, 64) // plain decimal, since an exponent isn't a command line argument
	default:
		return fmt.Sprint(v)
	}
}
