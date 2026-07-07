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

func TestTranscodeParamsRoundTrip(t *testing.T) {
	t.Parallel()

	in := transcodeParams{Codec: "opus", BitRate: 96, Channels: 2, SampleRate: 44100}
	out, err := decodeTranscodeParams(encodeTranscodeParams(in))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out != in {
		t.Errorf("round trip: got %+v want %+v", out, in)
	}
	if _, err := decodeTranscodeParams("not base64!!"); err == nil {
		t.Errorf("expected error decoding garbage")
	}
}

func TestDecideDirectPlay(t *testing.T) {
	t.Parallel()

	track := &db.Track{ID: 5, Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
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
	// direct play still hands out a token so the client can fetch raw bytes from getTranscodeStream. the
	// token is bound to the media it was decided for.
	tp, err := decodeTranscodeParams(d.TranscodeParams)
	if err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if !tp.DirectPlay || tp.Codec != "" || tp.MediaID != "tr-5" {
		t.Errorf("expected a direct play token bound to tr-5, got %+v", tp)
	}
	// audioBitrate is reported in bps
	if d.SourceStream.Container != "flac" || d.SourceStream.AudioBitDepth != 16 || d.SourceStream.AudioBitRateBPS != 900_000 {
		t.Errorf("unexpected source stream %+v", d.SourceStream)
	}
}

func TestDecideTranscodeCodecUnsupported(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.wma", Codec: "wmav2", Channels: 2, Bitrate: 192}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"mp3", "flac"}, AudioCodecs: []string{"mp3", "flac"}}},
		// aac is listed first but gonic can't produce it, so opus must be chosen
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "mp4", AudioCodec: "aac", Protocol: "http"}, {Container: "ogg", AudioCodec: "opus", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if d.CanDirectPlay || !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if want := []string{reasonContainer}; len(d.TranscodeReason) != 1 || d.TranscodeReason[0] != want[0] {
		t.Errorf("reasons: got %v want %v", d.TranscodeReason, want)
	}
	if d.TranscodeStream.Codec != "opus" {
		t.Errorf("target codec: got %q want opus", d.TranscodeStream.Codec)
	}
	tp, err := decodeTranscodeParams(d.TranscodeParams)
	if err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if tp.Codec != "opus" {
		t.Errorf("token codec: got %q want opus", tp.Codec)
	}
}

// container+codec are directplayable, but the global bitrate cap forces a transcode. the lossy source's
// bitrate is kept, capped down by the lower maxTranscodingAudioBitrate. all wire bitrates are bps.
func TestDecideTranscodeBitrateCap(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.mp3", Codec: "mp3", Channels: 2, Bitrate: 320}
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
	if d.TranscodeStream.AudioBitRateBPS != 64_000 { // 64 kbps in bps
		t.Errorf("target bitrate: got %d want 64000", d.TranscodeStream.AudioBitRateBPS)
	}
	tp, _ := decodeTranscodeParams(d.TranscodeParams)
	if tp.BitRate != 64 { // token stays in kbps
		t.Errorf("token bitrate: got %d want 64", tp.BitRate)
	}
}

func TestDecideTranscodeNoSupportedProfile(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.wma", Codec: "wmav2", Channels: 2, Bitrate: 192}
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

// a client whose only direct-play profile allows the container+codec but caps channels below the source's
// must be told channels are the problem, then offered a transcode.
func TestDecideDirectPlayChannelsExceeded(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 6, Bitrate: 2000}
	info := spec.ClientInfo{
		DirectPlayProfiles:  []spec.DirectPlayProfile{{Containers: []string{"flac"}, AudioCodecs: []string{"flac"}, MaxAudioChannels: 2}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "ogg", AudioCodec: "opus", Protocol: "http", MaxAudioChannels: 2}},
	}

	d := decideTranscode(info, track, nil, "")
	if d.CanDirectPlay || !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if want := reasonAudioChannels; len(d.TranscodeReason) != 1 || d.TranscodeReason[0] != want {
		t.Errorf("reasons: got %v want [%s]", d.TranscodeReason, want)
	}
	if d.TranscodeStream.AudioChannels != 2 { // downmixed to the profile's channel cap
		t.Errorf("target channels: got %d want 2", d.TranscodeStream.AudioChannels)
	}
	// the downmix must survive into the token so getTranscodeStream actually applies -ac 2
	if tp, _ := decodeTranscodeParams(d.TranscodeParams); tp.Channels != 2 {
		t.Errorf("token channels: got %d want 2", tp.Channels)
	}
}

// codecProfiles let a client that direct-plays mp3 still reject a too-high source samplerate.
func TestDecideCodecProfileSamplerateLimit(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.mp3", Codec: "mp3", Channels: 2, Bitrate: 128, SampleRate: 48000}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"mp3"}, AudioCodecs: []string{"mp3"}}},
		CodecProfiles: []spec.CodecProfile{{
			Type: "AudioCodec", Name: "mp3",
			Limitations: []spec.Limitation{{Name: "audioSamplerate", Comparison: "LessThanEqual", Values: []string{"44100"}, Required: true}},
		}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "mp3", AudioCodec: "mp3", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if d.CanDirectPlay {
		t.Fatalf("expected samplerate limit to block direct play, got %+v", d)
	}
	if want := reasonAudioSamplerate; len(d.TranscodeReason) != 1 || d.TranscodeReason[0] != want {
		t.Errorf("reasons: got %v want [%s]", d.TranscodeReason, want)
	}
	// the resample must survive into the token so getTranscodeStream actually applies -ar 44100
	if tp, _ := decodeTranscodeParams(d.TranscodeParams); tp.SampleRate != 44100 {
		t.Errorf("token samplerate: got %d want 44100", tp.SampleRate)
	}

	// a source already within the cap should direct-play
	track.SampleRate = 44100
	if d := decideTranscode(info, track, nil, ""); !d.CanDirectPlay {
		t.Errorf("source at the samplerate cap should direct play, got %+v", d)
	}
}

// a required samplerate limit opus can't meet must reject it for mp3, even though opus is listed first.
func TestDecideOpusSamplerateRejectsFallsBackToMP3(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 48000}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"mp3"}, AudioCodecs: []string{"mp3"}}},
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
		t.Errorf("target codec: got %q want mp3 (opus should be rejected by its samplerate limit)", d.TranscodeStream.Codec)
	}
}

// a podcast episode feeds the decision through the same db.AudioFile interface as a song, using the
// codec/channels probed and stored at download time.
func TestDecidePodcastEpisode(t *testing.T) {
	t.Parallel()

	ep := &db.PodcastEpisode{Filename: "episode.mp3", Codec: "mp3", Channels: 2, Bitrate: 128}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"mp3"}, AudioCodecs: []string{"mp3"}}},
	}

	d := decideTranscode(info, ep, nil, "")
	if !d.CanDirectPlay {
		t.Fatalf("expected direct play for mp3 podcast, got %+v", d)
	}
	if d.SourceStream.Codec != "mp3" || d.SourceStream.Container != "mp3" {
		t.Errorf("unexpected source stream %+v", d.SourceStream)
	}
}

// a codecProfile audioBitrate limitation is in bps, so a 128k mp3 exceeds a required 96000 cap and transcodes.
func TestDecideCodecProfileBitrateLimit(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.mp3", Codec: "mp3", Channels: 2, Bitrate: 128}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"mp3"}, AudioCodecs: []string{"mp3"}}},
		CodecProfiles: []spec.CodecProfile{{
			Type: "AudioCodec", Name: "mp3",
			Limitations: []spec.Limitation{{Name: "audioBitrate", Comparison: "LessThanEqual", Values: []string{"96000"}, Required: true}},
		}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "mp3", AudioCodec: "mp3", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if d.CanDirectPlay {
		t.Fatalf("expected bitrate limit to block direct play, got %+v", d)
	}
	if want := reasonAudioBitrate; len(d.TranscodeReason) != 1 || d.TranscodeReason[0] != want {
		t.Errorf("reasons: got %v want [%s]", d.TranscodeReason, want)
	}
}

// a lossy source keeps its own bitrate through a transcode, not the target profile's default.
func TestDecideTranscodeBitrateFromLossySource(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.wma", Codec: "wmav2", Channels: 2, Bitrate: 192}
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

// a lossless source's bitrate doesn't carry into a lossy target: it gets the profile default, or the client's cap.
func TestDecideTranscodeBitrateFromLosslessSource(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 48000, BitDepth: 16}
	info := spec.ClientInfo{
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "ogg", AudioCodec: "opus", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if d.TranscodeStream.AudioBitRateBPS != 96_000 { // opus profile default, not the flac's 900k
		t.Errorf("target bitrate: got %d want 96000", d.TranscodeStream.AudioBitRateBPS)
	}

	info.MaxTranscodingAudioBitRateBPS = 128_000
	if d := decideTranscode(info, track, nil, ""); d.TranscodeStream.AudioBitRateBPS != 128_000 {
		t.Errorf("target bitrate with cap: got %d want 128000", d.TranscodeStream.AudioBitRateBPS)
	}
}

// opus can't emit 44100, so the decision must report and pass on the 48000 it will actually produce.
func TestDecideOpusResampleReported(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
	info := spec.ClientInfo{
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "ogg", AudioCodec: "opus", Protocol: "http"}},
	}

	d := decideTranscode(info, track, nil, "")
	if !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if d.TranscodeStream.AudioSampleRate != 48000 {
		t.Errorf("target samplerate: got %d want 48000", d.TranscodeStream.AudioSampleRate)
	}
	if tp, _ := decodeTranscodeParams(d.TranscodeParams); tp.SampleRate != 48000 {
		t.Errorf("token samplerate: got %d want 48000", tp.SampleRate)
	}
}

// a 44100 source satisfies a required "samplerate <= 44100" limit, but opus would snap it up to 48000 and
// break it, so mp3 is chosen instead.
func TestDecideOpusSnappedRateRejectedByRequiredLimit(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
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

// libmp3lame only encodes mono/stereo, so a 6 channel source must be downmixed to 2 even when the client
// declares no channel limits at all.
func TestDecideMP3CodecChannelClamp(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 6, Bitrate: 2000, BitDepth: 16}
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

// a client declaring container "ogg" direct-plays a ".opus" file, since they share a MIME subtype.
func TestDecideContainerAliasDirectPlay(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.opus", Codec: "opus", Channels: 2, Bitrate: 128}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"ogg"}, AudioCodecs: []string{"opus"}}},
	}

	d := decideTranscode(info, track, nil, "")
	if !d.CanDirectPlay {
		t.Fatalf("expected direct play via ogg/opus container alias, got %+v", d)
	}
}

// a transcoding profile is only usable when gonic can produce both its codec and its container, over its
// protocol. the spec requires all three, so profiles missing one are skipped rather than guessed at.
func TestDecideTranscodeProfileMustMatch(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
	info := spec.ClientInfo{
		TranscodingProfiles: []spec.TranscodingProfile{
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

// a user-configured transcode for the client denies direct play the client would otherwise get, as long as
// the client declared it can accept the forced format. the token must carry the user profile's name so
// getTranscodeStream uses it (e.g. a replaygain variant) instead of the base codec profile.
func TestDecideForcedTranscode(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, BitDepth: 16}
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

// a user-configured format default supplies the profile behind a negotiated codec, like /stream's format
// param does: the token carries the profile's name and its bitrate acts as the default for lossless sources.
func TestDecideFormatDefaultTranscode(t *testing.T) {
	t.Parallel()

	// 6 channels at 44100, so the token must carry a downmix and resample for the user profile to apply
	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 6, SampleRate: 44100, Bitrate: 900, BitDepth: 16}
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

// the spec asks for one transcodeReason per direct play profile, in order, even when they repeat.
func TestDecideOneReasonPerDirectPlayProfile(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 6, Bitrate: 900, BitDepth: 16}
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

// a limitation that isn't required is applied where it can be, but never rejects a profile.
func TestDecideUnmetOptionalLimitation(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
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

// a required limitation outside what the spec defines can't be met, so it blocks rather than passes.
func TestDecideInvalidLimitationBlocks(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, BitDepth: 16}
	for _, lim := range []spec.Limitation{
		{Name: "audioChannels", Comparison: "LessThan", Values: []string{"8"}, Required: true},
		{Name: "audioChannels", Comparison: "LessThanEqual", Values: []string{"two"}, Required: true},
		{Name: "audioChannels", Comparison: "LessThanEqual", Required: true},
	} {
		info := spec.ClientInfo{
			DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"flac"}}},
			CodecProfiles:      []spec.CodecProfile{{Type: "AudioCodec", Name: "flac", Limitations: []spec.Limitation{lim}}},
		}
		if d := decideTranscode(info, track, nil, ""); d.CanDirectPlay {
			t.Errorf("%+v: expected no direct play", lim)
		}
	}
}

func TestContainsFormat(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		containers []string
		ext        string
		want       bool
	}{
		{[]string{"mp3"}, "mp3", true},
		{[]string{"mp3"}, "MP3", true},
		{[]string{"mp3", "flac"}, "flac", true},
		{[]string{"ogg"}, "opus", true},
		{[]string{"mp4"}, "m4a", true},
		{[]string{"flac"}, "mp3", false},
		{[]string{"audio/ogg"}, "opus", false},
		{[]string{""}, "opus", false},
	} {
		if got := containsFormat(tc.containers, tc.ext); got != tc.want {
			t.Errorf("containsFormat(%q, %q): got %v want %v", tc.containers, tc.ext, got, tc.want)
		}
	}
}

// a transcode pref is policy, not capability, so when it can't produce a playable stream the negotiation
// must fall back to what the client can actually take rather than failing outright.
func TestDecideForcedTranscodeFallsBackWhenUnplayable(t *testing.T) {
	t.Parallel()

	// opus can't emit 44100 and the client requires it, so the pref's format is rejected after narrowing
	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, SampleRate: 44100, Bitrate: 900, BitDepth: 16}
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

// a hi-res flac blocked from direct play by a samplerate limit can be resampled to flac rather than pushed
// to a lossy codec. lossless targets carry no bitrate, in the response or the token.
func TestDecideFlacResample(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 2000, SampleRate: 96000, BitDepth: 24}
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

// a 24 bit flac against a required "bitdepth <= 16" limit transcodes to 16 bit flac, and the token carries
// the conversion.
func TestDecideFlacBitDepthLimit(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 24}
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

// flac stores no depth at or below 8, so a required "bitdepth <= 8" limit rejects it for the next profile.
func TestDecideFlacBitDepthUnreachable(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 24}
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

// dsd is 1 bit, below any depth flac stores, so it snaps up to flac's lowest rather than being rejected. the
// token carries only what negotiation changed: dsd is scanned at 2822400, which flac can't encode, while ffmpeg
// decodes it at 352800, which flac can.
func TestDecideDSDToFlac(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.dsf", Codec: "dsd", Channels: 2, Bitrate: 5645, SampleRate: 2822400, BitDepth: 1}
	info := spec.ClientInfo{TranscodingProfiles: []spec.TranscodingProfile{{Container: "flac", AudioCodec: "flac", Protocol: "http"}, {Container: "mp3", AudioCodec: "mp3", Protocol: "http"}}}

	d := decideTranscode(info, track, nil, "")
	if !d.CanTranscode || d.TranscodeStream.Codec != "flac" || d.TranscodeStream.AudioBitDepth != 16 {
		t.Fatalf("expected 16 bit flac transcode, got %+v", d.TranscodeStream)
	}
	if tp, _ := decodeTranscodeParams(d.TranscodeParams); tp.BitDepth != 16 || tp.SampleRate != 0 || tp.Channels != 0 {
		t.Errorf("token: got %+v want bd=16 and nothing else changed", tp)
	}
}

// a lossy source never transcodes to a lossless target, so flac is rejected and the next profile serves.
func TestDecideLossyToLosslessRejected(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.mp3", Codec: "mp3", Channels: 2, Bitrate: 192, SampleRate: 44100}
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

// a lossless target can't have its bitrate capped, so a source exceeding the client's cap rejects flac.
func TestDecideFlacRejectedByBitrateCap(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
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

// same rejection through a codecProfile audioBitrate limitation rather than a global cap.
func TestDecideFlacRejectedByBitrateLimitation(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
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

// a forced transcode the client can't accept falls back to normal negotiation instead of failing playback.
func TestDecideForcedTranscodeUnsupportedFallsBack(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, BitDepth: 16}
	info := spec.ClientInfo{
		DirectPlayProfiles:  []spec.DirectPlayProfile{{Containers: []string{"flac"}, AudioCodecs: []string{"flac"}}},
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "ogg", AudioCodec: "opus", Protocol: "http"}},
	}

	if d := decideTranscode(info, track, nil, "mp3"); !d.CanDirectPlay {
		t.Errorf("expected fallback to direct play when the client can't accept the forced format, got %+v", d)
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

	reader := propsReader{Properties: tags.Properties{Codec: "flac", Channels: 2, SampleRate: 44100, BitDepth: 24}}
	require.NoError(t, backfillAudioProps(m.DB(), reader, &track))

	require.Equal(t, "flac", track.Codec)
	require.Equal(t, 24, track.BitDepth)

	var got db.Track
	require.NoError(t, m.DB().Preload("Album").First(&got, track.ID).Error)
	require.Equal(t, "flac", got.Codec)
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
