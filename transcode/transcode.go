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

var BaseProfiles = map[string]Profile{
	"mp3":  MP3,
	"opus": Opus,
	"flac": FLAC,
}

// BaseProfileFor resolves a requested format name to a base profile and its codec, matching the profile's
// suffix or its MIME subtype -- so "ogg" gets the opus profile
func BaseProfileFor(format string) (string, Profile, bool) {
	format = strings.ToLower(format)
	for codec, p := range BaseProfiles {
		if format == codec || "audio/"+format == p.MIME() {
			return codec, p, true
		}
	}
	return "", Profile{}, false
}

// Encoder describes the ffmpeg encoder behind a profile suffix. libmp3lame and libopus only encode discrete
// sample rates and cap their channel counts; other -ar or -ac values hard-fail
type Encoder struct {
	MaxChannels int
	SampleRates []int
}

//nolint:gochecknoglobals
var Encoders = map[string]Encoder{
	"mp3":  {MaxChannels: 2, SampleRates: []int{8000, 11025, 12000, 16000, 22050, 24000, 32000, 44100, 48000}},
	"opus": {MaxChannels: 8, SampleRates: []int{8000, 12000, 16000, 24000, 48000}},
	"flac": {MaxChannels: 8},
}

// NearestSampleRate snaps a desired rate to the encoder's discrete rates, preferring the smallest rate at or
// above it so we never downsample more than needed. an encoder with no listed rates resamples freely.
func (e Encoder) NearestSampleRate(rate int) int {
	if len(e.SampleRates) == 0 || slices.Contains(e.SampleRates, rate) {
		return rate
	}
	best := 0
	for _, s := range e.SampleRates {
		switch {
		case s >= rate && (best < rate || s < best):
			best = s
		case s < rate && best < rate && s > best:
			best = s
		}
	}
	return best
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

// Store as simple strings, since we may let the user provide their own profiles soon
var (
	MP3    = NewProfile("audio/mpeg", "mp3", 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -ac <channels> -ar <samplerate> -b:a <bitrate> -c:a libmp3lame -f mp3 -`)
	MP3320 = NewProfile("audio/mpeg", "mp3", 320, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libmp3lame -f mp3 -`)
	MP3RG  = NewProfile("audio/mpeg", "mp3", 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libmp3lame -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f mp3 -`)

	Opus       = NewProfile("audio/ogg", "opus", 96, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -ac <channels> -ar <samplerate> -b:a <bitrate> -c:a libopus -vbr constrained -f opus -`)
	OpusRG     = NewProfile("audio/ogg", "opus", 96, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr constrained -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)
	OpusRGLoud = NewProfile("audio/ogg", "opus", 96, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr constrained -af "aresample=96000:resampler=soxr, volume=replaygain=track:replaygain_preamp=15dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)

	Opus128       = NewProfile("audio/ogg", "opus", 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr constrained -f opus -`)
	Opus128RG     = NewProfile("audio/ogg", "opus", 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr constrained -af "volume=replaygain=track:replaygain_preamp=6dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)
	Opus128RGLoud = NewProfile("audio/ogg", "opus", 128, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr constrained -af "aresample=96000:resampler=soxr, volume=replaygain=track:replaygain_preamp=15dB:replaygain_noclip=0, alimiter=level=disabled, asidedata=mode=delete:type=REPLAYGAIN" -metadata replaygain_album_gain= -metadata replaygain_album_peak= -metadata replaygain_track_gain= -metadata replaygain_track_peak= -metadata r128_album_gain= -metadata r128_track_gain= -f opus -`)

	Opus192 = NewProfile("audio/ogg", "opus", 192, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -b:a <bitrate> -c:a libopus -vbr constrained -f opus -`)

	// lossless, so no bitrate. for resampling or bit depth conversion when a client can't play the source as-is
	FLAC = NewProfile("audio/flac", "flac", 0, `ffmpeg -v 0 -i <file> -ss <seek> -map 0:a:0 -vn -ac <channels> -ar <samplerate> <sampleformat> -c:a flac -f flac -`)

	PCM16le = NewProfile("audio/wav", "wav", 0, `ffmpeg -v 0 -i <file> -ss <seek> -c:a pcm_s16le -ac 2 -ar 48000 -f s16le -`)
)

type BitRate uint // kilobits/s

type Profile struct {
	bitrate    BitRate // the default bitrate, but the user can request a different one
	seek       time.Duration
	channels   int // 0 keeps the source's channel count
	sampleRate int // 0 keeps the source's sample rate
	bitDepth   int // 0 keeps the source's bit depth
	mime       string
	suffix     string
	exec       string
}

func (p Profile) BitRate() BitRate    { return p.bitrate }
func (p Profile) Seek() time.Duration { return p.seek }
func (p Profile) Suffix() string      { return p.suffix }
func (p Profile) MIME() string        { return p.mime }

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

	return name, args, nil
}
