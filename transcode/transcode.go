// author: spijet (https://github.com/spijet/)
// author: sentriz (https://github.com/sentriz/)

//nolint:gochecknoglobals,goconst
package transcode

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strconv"
	"strings"
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

// BaseProfiles are the profiles the transcoding extension can offer, one per codec it can negotiate
var BaseProfiles = map[CodecName]Profile{}

func init() { //nolint:gochecknoinits
	for _, p := range []Profile{MP3, Opus, FLAC} {
		BaseProfiles[p.Codec().Name] = p
	}
}

// Store as simple strings, since we may let the user provide their own profiles soon
var (
	MP3    = NewProfile(CodecMP3, 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -ac <channels> -ar <samplerate> -b:a <bitrate> -c:a libmp3lame -f mp3 -`)
	MP3320 = NewProfile(CodecMP3, 320, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -ac <channels> -ar <samplerate> -b:a <bitrate> -c:a libmp3lame -f mp3 -`)
	MP3RG  = NewProfile(CodecMP3, 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -ac <channels> -ar <samplerate> -b:a <bitrate> -c:a libmp3lame -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f mp3 -`)

	Opus       = NewProfile(CodecOpus, 96, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -ac <channels> -ar <samplerate> -b:a <bitrate> -c:a libopus -vbr constrained -f opus -`)
	OpusRG     = NewProfile(CodecOpus, 96, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -ac <channels> -ar <samplerate> -b:a <bitrate> -c:a libopus -vbr constrained -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)
	OpusRGLoud = NewProfile(CodecOpus, 96, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -ac <channels> -ar <samplerate> -b:a <bitrate> -c:a libopus -vbr constrained -af "aresample=96000:resampler=soxr, volume=replaygain=track:replaygain_preamp=15dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)

	Opus128       = NewProfile(CodecOpus, 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -ac <channels> -ar <samplerate> -b:a <bitrate> -c:a libopus -vbr constrained -f opus -`)
	Opus128RG     = NewProfile(CodecOpus, 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -ac <channels> -ar <samplerate> -b:a <bitrate> -c:a libopus -vbr constrained -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)
	Opus128RGLoud = NewProfile(CodecOpus, 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -ac <channels> -ar <samplerate> -b:a <bitrate> -c:a libopus -vbr constrained -af "aresample=96000:resampler=soxr, volume=replaygain=track:replaygain_preamp=15dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)

	Opus192 = NewProfile(CodecOpus, 192, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -ac <channels> -ar <samplerate> -b:a <bitrate> -c:a libopus -vbr constrained -f opus -`)

	// lossless, so no bitrate. for resampling or bit depth conversion when a client can't play the source as-is
	FLAC = NewProfile(CodecFLAC, 0, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -ac <channels> -ar <samplerate> <sampleformat> -c:a flac -f flac -`)

	PCM16le = NewProfile(CodecPCM, 0, `ffmpeg -v 0 -i <file> -ss <seek> -c:a pcm_s16le -ac 2 -ar 48000 -f s16le -`)
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

func init() { //nolint:gochecknoinits
	for _, c := range []Codec{CodecMP3, CodecOpus, CodecFLAC, CodecPCM} {
		Codecs[c.Name] = c
	}
}

// NearestSampleRate snaps a desired rate to the codec's discrete rates, preferring the smallest rate at or
// above it so we never downsample more than needed. a codec with no listed rates resamples freely.
func (c Codec) NearestSampleRate(rate int) int {
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

// NearestBitDepth snaps a desired depth down to one the codec can store, since bits can't be invented. 0
// means the codec has nothing to snap to, or the depth is below anything it stores.
func (c Codec) NearestBitDepth(depth int) int {
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

var (
	ErrNoProfileParts = fmt.Errorf("not enough profile parts")
	ErrNoPlaceholder  = fmt.Errorf("profile has no placeholder for a requested property")
)

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

	var seen []string
	var args []string
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "<") {
			seen = append(seen, p)
		}
		switch p {
		case "<file>":
			args = append(args, in)
		case "<seek>":
			args = append(args, fmt.Sprintf("%dus", profile.Seek().Microseconds()))
		case "<bitrate>":
			args = append(args, fmt.Sprintf("%dk", profile.BitRate()))
		case "<channels>":
			args = append(args, strconv.Itoa(profile.channels)) // 0 -> ffmpeg keeps the source's channels
		case "<samplerate>":
			args = append(args, strconv.Itoa(profile.sampleRate)) // 0 -> ffmpeg keeps the source's rate
		case "<sampleformat>":
			// expands to a -sample_fmt pair or nothing, since -sample_fmt has no "keep the source's" sentinel
			switch {
			case profile.bitDepth == 0:
			case profile.bitDepth <= 16:
				args = append(args, "-sample_fmt", "s16")
			default:
				args = append(args, "-sample_fmt", "s32") // flac stores 24 bit from s32 input
			}
		default:
			args = append(args, p)
		}
	}

	// a property with nowhere to expand would be silently dropped, serving audio that doesn't match what the
	// caller (and the transcoding extension's decision) promised
	for _, missing := range []struct {
		placeholder string
		set         bool
	}{
		{"<channels>", profile.channels != 0},
		{"<samplerate>", profile.sampleRate != 0},
		{"<sampleformat>", profile.bitDepth != 0},
	} {
		if missing.set && !slices.Contains(seen, missing.placeholder) {
			return "", nil, fmt.Errorf("%w: %s", ErrNoPlaceholder, missing.placeholder)
		}
	}

	return name, args, nil
}
