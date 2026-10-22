package ctrlsubsonic

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/mockfs"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/tags"
	"go.senan.xyz/gonic/transcode"
)

func TestDecideDirectPlay(t *testing.T) {
	t.Parallel()

	track := &db.Track{ID: 5, Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"flac", "mp3"}, AudioCodecs: []string{"flac"}}},
	}

	d := decideTranscode(info, track, nil, "")
	if !d.CanDirectPlay || d.CanTranscode {
		t.Fatalf("expected direct play, got %+v", d)
	}
	if d.TranscodeStream != nil {
		t.Errorf("direct play should not carry a transcode stream: %+v", d)
	}
	tp, err := decodeTranscodeParams(d.TranscodeParams)
	if err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if !tp.DirectPlay || tp.Codec != "" || tp.MediaID != "tr-5" {
		t.Errorf("expected a direct play token bound to tr-5, got %+v", tp)
	}
	if d.SourceStream.Container != "flac" || d.SourceStream.AudioBitDepth != 16 || d.SourceStream.AudioBitRateBPS != 900_000 {
		t.Errorf("unexpected source stream %+v", d.SourceStream)
	}
}

func TestDecideTranscodeBitrateCap(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.mp3", Codec: "mp3", Container: "mp3", Channels: 2, Bitrate: 320}
	info := spec.ClientInfo{
		MaxAudioBitRateBPS:            128_000,
		MaxTranscodingAudioBitRateBPS: 64_000,
		DirectPlayProfiles:            []spec.DirectPlayProfile{{Containers: []string{"mp3"}, AudioCodecs: []string{"mp3"}}},
		TranscodingProfiles:           []spec.TranscodingProfile{{Container: "mp3", AudioCodec: "mp3", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if want := reasonAudioBitrate; len(d.TranscodeReason) != 1 || d.TranscodeReason[0] != want {
		t.Errorf("reasons: got %v want [%s]", d.TranscodeReason, want)
	}
	if d.TranscodeStream.AudioBitRateBPS != 64_000 {
		t.Errorf("target bitrate: got %d want 64000", d.TranscodeStream.AudioBitRateBPS)
	}
	tp, _ := decodeTranscodeParams(d.TranscodeParams)
	if tp.BitRate != 64 {
		t.Errorf("token bitrate: got %d want 64", tp.BitRate)
	}
}

func TestDecideTranscodeNoSupportedProfile(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.wma", Codec: "wmav2", Container: "asf", Channels: 2, Bitrate: 192}
	info := spec.ClientInfo{
		DirectPlayProfiles:  []spec.DirectPlayProfile{{Containers: []string{"mp3"}, AudioCodecs: []string{"mp3"}}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "mp4", AudioCodec: "aac", Protocol: "http"}}, // gonic can't produce aac
	}

	d := decideTranscode(info, track, nil, "")
	if d.CanDirectPlay || d.CanTranscode {
		t.Fatalf("expected neither direct play nor transcode, got %+v", d)
	}
	if d.ErrorReason == "" {
		t.Errorf("expected an error reason")
	}
}

func TestDecideTranscodeBitrateFromLossySource(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.wma", Codec: "wmav2", Container: "asf", Channels: 2, Bitrate: 192}
	info := spec.ClientInfo{
		DirectPlayProfiles:  []spec.DirectPlayProfile{{Containers: []string{"mp3"}, AudioCodecs: []string{"mp3"}}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "ogg", AudioCodec: "opus", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if d.TranscodeStream.AudioBitRateBPS != 192_000 {
		t.Errorf("target bitrate: got %d want 192000", d.TranscodeStream.AudioBitRateBPS)
	}
	if tp, _ := decodeTranscodeParams(d.TranscodeParams); tp.BitRate != 192 {
		t.Errorf("token bitrate: got %d want 192", tp.BitRate)
	}
}

func TestDecideTranscodeBitrateFromLosslessSource(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 2, Bitrate: 900, SampleRate: 48000, BitDepth: 16}
	info := spec.ClientInfo{
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "ogg", AudioCodec: "opus", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if d.TranscodeStream.AudioBitRateBPS != 96_000 { // opus profile default
		t.Errorf("target bitrate: got %d want 96000", d.TranscodeStream.AudioBitRateBPS)
	}

	info.MaxTranscodingAudioBitRateBPS = 128_000
	if d := decideTranscode(info, track, nil, ""); d.TranscodeStream.AudioBitRateBPS != 128_000 {
		t.Errorf("target bitrate with cap: got %d want 128000", d.TranscodeStream.AudioBitRateBPS)
	}
}

// opus snaps 44100 up to 48000, breaking the limit the source meets
func TestDecideOpusSnappedRateRejectedByRequiredLimit(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
	info := spec.ClientInfo{
		CodecProfiles: []spec.CodecProfile{{
			Type: "AudioCodec", Name: "opus",
			Limitations: []spec.Limitation{{Name: "audioSamplerate", Comparison: "LessThanEqual", Values: []string{"44100"}, Required: true}},
		}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "ogg", AudioCodec: "opus", Protocol: "http"}, {Container: "mp3", AudioCodec: "mp3", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if d.TranscodeStream.Codec != "mp3" {
		t.Errorf("target codec: got %q want mp3 (opus's snapped 48000 violates the required limit)", d.TranscodeStream.Codec)
	}
	if d.TranscodeStream.AudioSampleRate != 44100 {
		t.Errorf("target samplerate: got %d want 44100", d.TranscodeStream.AudioSampleRate)
	}
}

// libmp3lame only encodes mono and stereo
func TestDecideMP3CodecChannelClamp(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 6, Bitrate: 2000, BitDepth: 16}
	info := spec.ClientInfo{
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "mp3", AudioCodec: "mp3", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if d.TranscodeStream.AudioChannels != 2 {
		t.Errorf("target channels: got %d want 2", d.TranscodeStream.AudioChannels)
	}
	if tp, _ := decodeTranscodeParams(d.TranscodeParams); tp.Channels != 2 {
		t.Errorf("token channels: got %d want 2", tp.Channels)
	}
}

func TestDecideTranscodeProfileMustMatch(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
	info := spec.ClientInfo{
		TranscodingProfiles: []spec.TranscodingProfile{
			{Container: "mp4", AudioCodec: "aac", Protocol: "http"},
			{Container: "mp4", AudioCodec: "flac", Protocol: "http"},
			{Container: "ogg", AudioCodec: "opus", Protocol: "hls"},
			{Container: "ogg", AudioCodec: "opus"},
			{Container: "ogg", Protocol: "http"},
			{AudioCodec: "opus", Protocol: "http"},
			{Container: "ogg", AudioCodec: "OPUS", Protocol: "http"},
			{Container: "mp3", AudioCodec: "mp3", Protocol: "http"},
		},
	}

	d := decideTranscode(info, track, nil, "")
	if !d.CanTranscode || d.TranscodeStream.Codec != "mp3" || d.TranscodeStream.Container != "mp3" {
		t.Fatalf("expected only the last profile to match, got %+v", d.TranscodeStream)
	}
}

func TestDecideForcedTranscode(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 2, Bitrate: 900, BitDepth: 16}
	info := spec.ClientInfo{
		DirectPlayProfiles:  []spec.DirectPlayProfile{{Containers: []string{"flac"}, AudioCodecs: []string{"flac"}}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "ogg", AudioCodec: "opus", Protocol: "http"}},
	}

	if d := decideTranscode(info, track, nil, ""); !d.CanDirectPlay {
		t.Fatalf("control: expected direct play without a forced profile, got %+v", d)
	}

	d := decideTranscode(info, track, nil, "opus_rg")
	if d.CanDirectPlay || !d.CanTranscode {
		t.Fatalf("expected forced transcode, got %+v", d)
	}
	if !slices.Equal(d.TranscodeReason, []string{reasonTranscodePreference}) {
		t.Errorf("reasons: got %v want [%s]", d.TranscodeReason, reasonTranscodePreference)
	}
	if d.TranscodeStream.AudioBitRateBPS != 96_000 { // capped to the forced profile's bitrate
		t.Errorf("target bitrate: got %d want 96000", d.TranscodeStream.AudioBitRateBPS)
	}
	tp, err := decodeTranscodeParams(d.TranscodeParams)
	if err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if tp.Profile != "opus_rg" || tp.Codec != "opus" {
		t.Errorf("token: got %+v want profile opus_rg codec opus", tp)
	}
}

func TestDecideFormatDefaultTranscode(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 6, SampleRate: 44100, Bitrate: 900, BitDepth: 16}
	info := spec.ClientInfo{
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "ogg", AudioCodec: "opus", Protocol: "http", MaxAudioChannels: 2}},
	}

	d := decideTranscode(info, track, map[transcode.CodecName]string{transcode.CodecOpus.Name: "opus_192"}, "")
	if !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if d.TranscodeStream.AudioBitRateBPS != 192_000 {
		t.Errorf("target bitrate: got %d want 192000", d.TranscodeStream.AudioBitRateBPS)
	}
	tp, err := decodeTranscodeParams(d.TranscodeParams)
	if err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if tp.Profile != "opus_192" || tp.Codec != "opus" {
		t.Errorf("token: got %+v want profile opus_192 codec opus", tp)
	}
	if tp.Channels != 2 || tp.SampleRate != 48000 {
		t.Errorf("token: got %d channels at %d want 2 at 48000", tp.Channels, tp.SampleRate)
	}
}

func TestDecideOneReasonPerDirectPlayProfile(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 6, Bitrate: 900, BitDepth: 16}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{
			{Containers: []string{"mp3"}},
			{Containers: []string{"flac"}, MaxAudioChannels: 2},
			{Containers: []string{"flac"}, MaxAudioChannels: 2},
		},
	}

	d := decideTranscode(info, track, nil, "")
	want := []string{reasonContainer, reasonAudioChannels, reasonAudioChannels}
	if !slices.Equal(d.TranscodeReason, want) {
		t.Errorf("reasons: got %v want %v", d.TranscodeReason, want)
	}

	d = decideTranscode(spec.ClientInfo{DirectPlayProfiles: info.DirectPlayProfiles, MaxAudioBitRateBPS: 320_000}, track, nil, "")
	want = []string{reasonAudioBitrate, reasonAudioBitrate, reasonAudioBitrate}
	if !slices.Equal(d.TranscodeReason, want) {
		t.Errorf("reasons over the global cap: got %v want %v", d.TranscodeReason, want)
	}
}

func TestDecideUnmetOptionalLimitation(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
	info := spec.ClientInfo{
		CodecProfiles: []spec.CodecProfile{{
			Type: "AudioCodec", Name: "opus",
			Limitations: []spec.Limitation{{Name: "audioSamplerate", Comparison: "GreaterThanEqual", Values: []string{"96000"}}},
		}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "ogg", AudioCodec: "opus", Protocol: "http"}},
	}

	if d := decideTranscode(info, track, nil, ""); !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
}

func TestDecideForcedTranscodeFallsBackWhenUnplayable(t *testing.T) {
	t.Parallel()

	// opus can't emit the 44100 the client requires
	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 2, SampleRate: 44100, Bitrate: 900, BitDepth: 16}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"flac"}, AudioCodecs: []string{"flac"}}},
		CodecProfiles: []spec.CodecProfile{{
			Type: "AudioCodec", Name: "opus",
			Limitations: []spec.Limitation{{Name: "audioSamplerate", Comparison: "LessThanEqual", Values: []string{"44100"}, Required: true}},
		}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "ogg", AudioCodec: "opus", Protocol: "http"}, {Container: "mp3", AudioCodec: "mp3", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "opus_192")
	if !d.CanDirectPlay && !d.CanTranscode {
		t.Fatalf("pref made the track unplayable: %+v", d)
	}
	if d.ErrorReason != "" {
		t.Errorf("error reason: got %q want none", d.ErrorReason)
	}
}

func TestDecideFlacResample(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 2, Bitrate: 2000, SampleRate: 96000, BitDepth: 24}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"flac"}, AudioCodecs: []string{"flac"}}},
		CodecProfiles: []spec.CodecProfile{{
			Type: "AudioCodec", Name: "flac",
			Limitations: []spec.Limitation{{Name: "audioSamplerate", Comparison: "LessThanEqual", Values: []string{"48000"}, Required: true}},
		}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "flac", AudioCodec: "flac", Protocol: "http"}, {Container: "ogg", AudioCodec: "opus", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if d.CanDirectPlay || !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if d.TranscodeStream.Codec != "flac" || d.TranscodeStream.AudioSampleRate != 48000 {
		t.Errorf("target: got %s@%d want flac@48000", d.TranscodeStream.Codec, d.TranscodeStream.AudioSampleRate)
	}
	if d.TranscodeStream.AudioBitRateBPS != 0 {
		t.Errorf("lossless target bitrate: got %d want 0", d.TranscodeStream.AudioBitRateBPS)
	}
	tp, err := decodeTranscodeParams(d.TranscodeParams)
	if err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if tp.Codec != "flac" || tp.SampleRate != 48000 || tp.BitRate != 0 || tp.BitDepth != 0 {
		t.Errorf("token: got %+v want flac sr=48000 no bitrate no bitdepth", tp)
	}
}

func TestDecideFlacBitDepthLimit(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 24}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"flac"}, AudioCodecs: []string{"flac"}}},
		CodecProfiles: []spec.CodecProfile{{
			Type: "AudioCodec", Name: "flac",
			Limitations: []spec.Limitation{{Name: "audioBitdepth", Comparison: "LessThanEqual", Values: []string{"16"}, Required: true}},
		}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "flac", AudioCodec: "flac", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if d.CanDirectPlay || !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if want := reasonAudioBitdepth; len(d.TranscodeReason) != 1 || d.TranscodeReason[0] != want {
		t.Errorf("reasons: got %v want [%s]", d.TranscodeReason, want)
	}
	if d.TranscodeStream.AudioBitDepth != 16 {
		t.Errorf("target bitdepth: got %d want 16", d.TranscodeStream.AudioBitDepth)
	}
	if tp, _ := decodeTranscodeParams(d.TranscodeParams); tp.BitDepth != 16 {
		t.Errorf("token bitdepth: got %d want 16", tp.BitDepth)
	}
}

// flac stores no depth below 16
func TestDecideFlacBitDepthUnreachable(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 24}
	info := spec.ClientInfo{
		CodecProfiles: []spec.CodecProfile{{
			Type: "AudioCodec", Name: "flac",
			Limitations: []spec.Limitation{{Name: "audioBitdepth", Comparison: "LessThanEqual", Values: []string{"8"}, Required: true}},
		}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "flac", AudioCodec: "flac", Protocol: "http"}, {Container: "mp3", AudioCodec: "mp3", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if !d.CanTranscode || d.TranscodeStream.Codec != "mp3" {
		t.Fatalf("expected mp3 transcode, got %+v", d)
	}
}

// dsd scans as 1 bit at 2822400 Hz. ffmpeg decodes it at a rate flac takes, so only the depth is passed on
func TestDecideDSDToFlac(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.dsf", Codec: "dsd", Container: "dsf", Channels: 2, Bitrate: 5645, SampleRate: 2822400, BitDepth: 1}
	info := spec.ClientInfo{TranscodingProfiles: []spec.TranscodingProfile{{Container: "flac", AudioCodec: "flac", Protocol: "http"}, {Container: "mp3", AudioCodec: "mp3", Protocol: "http"}}}

	d := decideTranscode(info, track, nil, "")
	if !d.CanTranscode || d.TranscodeStream.Codec != "flac" || d.TranscodeStream.AudioBitDepth != 16 {
		t.Fatalf("expected 16 bit flac transcode, got %+v", d.TranscodeStream)
	}
	if tp, _ := decodeTranscodeParams(d.TranscodeParams); tp.BitDepth != 16 || tp.SampleRate != 0 || tp.Channels != 0 {
		t.Errorf("token: got %+v want bd=16 and nothing else changed", tp)
	}
}

func TestDecideLossyToLosslessRejected(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.mp3", Codec: "mp3", Container: "mp3", Channels: 2, Bitrate: 192, SampleRate: 44100}
	info := spec.ClientInfo{
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "flac", AudioCodec: "flac", Protocol: "http"}, {Container: "ogg", AudioCodec: "opus", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if d.TranscodeStream.Codec != "opus" {
		t.Errorf("target codec: got %q want opus (flac must be rejected for a lossy source)", d.TranscodeStream.Codec)
	}
}

// a lossless bitrate can't be capped
func TestDecideFlacRejectedByBitrateCap(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
	info := spec.ClientInfo{
		MaxAudioBitRateBPS:  320_000,
		DirectPlayProfiles:  []spec.DirectPlayProfile{{Containers: []string{"flac"}, AudioCodecs: []string{"flac"}}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "flac", AudioCodec: "flac", Protocol: "http"}, {Container: "ogg", AudioCodec: "opus", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if d.TranscodeStream.Codec != "opus" || d.TranscodeStream.AudioBitRateBPS != 320_000 {
		t.Errorf("target: got %s@%d want opus@320000", d.TranscodeStream.Codec, d.TranscodeStream.AudioBitRateBPS)
	}
}

func TestDecideFlacRejectedByBitrateLimitation(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
	info := spec.ClientInfo{
		CodecProfiles: []spec.CodecProfile{{
			Type: "AudioCodec", Name: "flac",
			Limitations: []spec.Limitation{{Name: "audioBitrate", Comparison: "LessThanEqual", Values: []string{"500000"}, Required: true}},
		}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "flac", AudioCodec: "flac", Protocol: "http"}, {Container: "ogg", AudioCodec: "opus", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if d.TranscodeStream.Codec != "opus" {
		t.Errorf("target codec: got %q want opus (flac can't honor the bitrate limitation)", d.TranscodeStream.Codec)
	}
}

func TestDecideForcedTranscodeUnsupportedFallsBack(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Container: "flac", Channels: 2, Bitrate: 900, BitDepth: 16}
	info := spec.ClientInfo{
		DirectPlayProfiles:  []spec.DirectPlayProfile{{Containers: []string{"flac"}, AudioCodecs: []string{"flac"}}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "ogg", AudioCodec: "opus", Protocol: "http"}},
	}

	if d := decideTranscode(info, track, nil, "mp3"); !d.CanDirectPlay {
		t.Errorf("expected fallback to direct play when the client can't accept the forced format, got %+v", d)
	}
}

func TestAdjust(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		comparison string
		values     []string
		in         int
		want       int
		wantOK     bool
	}{
		{spec.ComparisonLessThanEqual, []string{"44100"}, 48000, 44100, true},
		{spec.ComparisonLessThanEqual, []string{"44100"}, 44100, 44100, true},
		{spec.ComparisonLessThanEqual, []string{"44100", "8000"}, 48000, 44100, true},
		{spec.ComparisonGreaterThanEqual, []string{"96000"}, 48000, 48000, false},
		{spec.ComparisonGreaterThanEqual, []string{"96000"}, 96000, 96000, true},
		{spec.ComparisonEquals, []string{"1", "2"}, 6, 2, true},
		{spec.ComparisonEquals, []string{"1", "2"}, 2, 2, true},
		{spec.ComparisonEquals, []string{"4"}, 2, 2, false},
		{spec.ComparisonNotEquals, []string{"6"}, 6, 6, false},
		{spec.ComparisonNotEquals, []string{"6"}, 2, 2, true},
		{spec.ComparisonLessThanEqual, []string{"2"}, 0, 0, true},
		{"LessThan", []string{"8"}, 6, 6, false},
		{spec.ComparisonLessThanEqual, []string{"two"}, 6, 6, false},
		{spec.ComparisonLessThanEqual, nil, 6, 6, false},
	} {
		got := tc.in
		ok := adjust(spec.Limitation{Comparison: tc.comparison, Values: tc.values}, &got)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("%s %v on %d: got %d %v want %d %v", tc.comparison, tc.values, tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestBackfillAudioPropsWritesOnlyItsColumns(t *testing.T) {
	t.Parallel()

	m := mockfs.New(t)
	m.AddItems()
	m.ScanAndClean()

	var track db.Track
	require.NoError(t, m.DB().Preload("Album").First(&track).Error)
	origTitle, origAlbumTitle := track.TagTitle, track.Album.TagTitle

	track.TagTitle = "stale title"
	track.Album.TagTitle = "stale album title"

	reader := propsReader{Codec: "flac", Container: "flac", Channels: 2, SampleRate: 44100, BitDepth: 24}
	require.NoError(t, backfillAudioProps(m.DB(), reader, &track))

	require.Equal(t, "flac", track.Codec)
	require.Equal(t, 24, track.BitDepth)

	var got db.Track
	require.NoError(t, m.DB().Preload("Album").First(&got, track.ID).Error)
	require.Equal(t, "flac", got.Codec)
	require.Equal(t, "flac", got.Container)
	require.Equal(t, 2, got.Channels)
	require.Equal(t, 44100, got.SampleRate)
	require.Equal(t, 24, got.BitDepth)
	require.Equal(t, origTitle, got.TagTitle)
	require.Equal(t, origAlbumTitle, got.Album.TagTitle)
}

type propsReader struct{ tags.Properties }

func (r propsReader) CanRead(string) bool { return true }
func (r propsReader) Read(string) (tags.Properties, map[string][]string, error) {
	return r.Properties, nil, nil
}
func (r propsReader) ReadCover(string) ([]byte, error) { return nil, nil }
