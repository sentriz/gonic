package ctrlsubsonic

import (
	"testing"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
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

	d := decideTranscode(info, track, "", nil)
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
		// aac is listed first but gonic can't produce it -- opus must be chosen, not the first entry
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "aac"}, {AudioCodec: "opus"}},
	}

	d := decideTranscode(info, track, "", nil)
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
		TranscodingProfiles:           []spec.TranscodingProfile{{AudioCodec: "mp3"}},
	}

	d := decideTranscode(info, track, "", nil)
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
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "aac"}}, // gonic can't produce aac
	}

	d := decideTranscode(info, track, "", nil)
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
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "opus", MaxAudioChannels: 2}},
	}

	d := decideTranscode(info, track, "", nil)
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

// codecProfiles let a client that direct-plays mp3 still reject a too-high source samplerate. the required
// LessThanEqual limitation must bite -- a source at exactly the limit would pass, so 48000 vs a 44100 cap differ.
func TestDecideCodecProfileSamplerateLimit(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.mp3", Codec: "mp3", Channels: 2, Bitrate: 128, SampleRate: 48000}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"mp3"}, AudioCodecs: []string{"mp3"}}},
		CodecProfiles: []spec.CodecProfile{{
			Type: "AudioCodec", Name: "mp3",
			Limitations: []spec.Limitation{{Name: "audioSamplerate", Comparison: "LessThanEqual", Values: []string{"44100"}, Required: true}},
		}},
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "mp3"}},
	}

	d := decideTranscode(info, track, "", nil)
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
	if d := decideTranscode(info, track, "", nil); !d.CanDirectPlay {
		t.Errorf("source at the samplerate cap should direct play, got %+v", d)
	}
}

// opus can't be re-rated to an arbitrary sample rate, so a required samplerate limit its output can't meet must
// reject the opus profile and fall through to mp3 rather than hand ffmpeg an unsupported -ar. opus is listed first,
// so a naive "first profile wins" would pick it -- the test only passes if the samplerate limit actually rejects opus.
func TestDecideOpusSamplerateRejectsFallsBackToMP3(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 48000}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"mp3"}, AudioCodecs: []string{"mp3"}}},
		CodecProfiles: []spec.CodecProfile{{
			Type: "AudioCodec", Name: "opus",
			Limitations: []spec.Limitation{{Name: "audioSamplerate", Comparison: "LessThanEqual", Values: []string{"44100"}, Required: true}},
		}},
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "opus"}, {AudioCodec: "mp3"}},
	}

	d := decideTranscode(info, track, "", nil)
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

	d := decideTranscode(info, ep, "", nil)
	if !d.CanDirectPlay {
		t.Fatalf("expected direct play for mp3 podcast, got %+v", d)
	}
	if d.SourceStream.Codec != "mp3" || d.SourceStream.Container != "mp3" {
		t.Errorf("unexpected source stream %+v", d.SourceStream)
	}
}

// a codecProfile audioBitrate limitation is compared directly against the source bitrate in bps -- no
// conversion. a 128k mp3 exceeding a required 96000 bps cap must be pushed to transcode.
func TestDecideCodecProfileBitrateLimit(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.mp3", Codec: "mp3", Channels: 2, Bitrate: 128}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"mp3"}, AudioCodecs: []string{"mp3"}}},
		CodecProfiles: []spec.CodecProfile{{
			Type: "AudioCodec", Name: "mp3",
			Limitations: []spec.Limitation{{Name: "audioBitrate", Comparison: "LessThanEqual", Values: []string{"96000"}, Required: true}},
		}},
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "mp3"}},
	}

	d := decideTranscode(info, track, "", nil)
	if d.CanDirectPlay {
		t.Fatalf("expected bitrate limit to block direct play, got %+v", d)
	}
	if want := reasonAudioBitrate; len(d.TranscodeReason) != 1 || d.TranscodeReason[0] != want {
		t.Errorf("reasons: got %v want [%s]", d.TranscodeReason, want)
	}
}

// a lossy source keeps its own bitrate through a transcode -- a 192k wma pushed to opus must come out at
// 192k, not opus's 96k profile default.
func TestDecideTranscodeBitrateFromLossySource(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.wma", Codec: "wmav2", Channels: 2, Bitrate: 192}
	info := spec.ClientInfo{
		DirectPlayProfiles:  []spec.DirectPlayProfile{{Containers: []string{"mp3"}, AudioCodecs: []string{"mp3"}}},
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "opus"}},
	}

	d := decideTranscode(info, track, "", nil)
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

// a lossless source (identified by its scanned bit depth) must not carry its huge bitrate into a lossy
// target -- with no client caps it gets the profile default, with a cap it gets the cap.
func TestDecideTranscodeBitrateFromLosslessSource(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 48000, BitDepth: 16}
	info := spec.ClientInfo{
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "opus"}},
	}

	d := decideTranscode(info, track, "", nil)
	if !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if d.TranscodeStream.AudioBitRateBPS != 96_000 { // opus profile default, not the flac's 900k
		t.Errorf("target bitrate: got %d want 96000", d.TranscodeStream.AudioBitRateBPS)
	}

	info.MaxTranscodingAudioBitRateBPS = 128_000
	if d := decideTranscode(info, track, "", nil); d.TranscodeStream.AudioBitRateBPS != 128_000 {
		t.Errorf("target bitrate with cap: got %d want 128000", d.TranscodeStream.AudioBitRateBPS)
	}
}

// opus can't emit 44100, so the decision must report the 48000 ffmpeg will actually produce, and pass it
// explicitly -- a naive "keep the source's rate" would advertise 44100 and lie to the client.
func TestDecideOpusResampleReported(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
	info := spec.ClientInfo{
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "opus"}},
	}

	d := decideTranscode(info, track, "", nil)
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

// a 44100 source satisfies a required "samplerate <= 44100" limit as-is, but opus would snap it up to 48000
// and violate it -- the opus profile must be rejected for mp3, which can emit 44100 directly.
func TestDecideOpusSnappedRateRejectedByRequiredLimit(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
	info := spec.ClientInfo{
		CodecProfiles: []spec.CodecProfile{{
			Type: "AudioCodec", Name: "opus",
			Limitations: []spec.Limitation{{Name: "audioSamplerate", Comparison: "LessThanEqual", Values: []string{"44100"}, Required: true}},
		}},
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "opus"}, {AudioCodec: "mp3"}},
	}

	d := decideTranscode(info, track, "", nil)
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
func TestDecideMP3EncoderChannelClamp(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 6, Bitrate: 2000, BitDepth: 16}
	info := spec.ClientInfo{
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "mp3"}},
	}

	d := decideTranscode(info, track, "", nil)
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

// container names match by MIME type too, so a client declaring container "ogg" direct-plays a ".opus"
// file -- strict extension comparison would force a pointless transcode.
func TestDecideContainerAliasDirectPlay(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.opus", Codec: "opus", Channels: 2, Bitrate: 128}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"ogg"}, AudioCodecs: []string{"opus"}}},
	}

	d := decideTranscode(info, track, "", nil)
	if !d.CanDirectPlay {
		t.Fatalf("expected direct play via ogg/opus container alias, got %+v", d)
	}
}

// a transcoding profile declaring only container "ogg" resolves to gonic's opus profile via its MIME type.
func TestDecideContainerOnlyOggTranscode(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.wma", Codec: "wmav2", Channels: 2, Bitrate: 192}
	info := spec.ClientInfo{
		TranscodingProfiles: []spec.TranscodingProfile{{Container: "ogg"}},
	}

	d := decideTranscode(info, track, "", nil)
	if !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if d.TranscodeStream.Codec != "opus" || d.TranscodeStream.Container != "ogg" {
		t.Errorf("target: got %s/%s want ogg/opus", d.TranscodeStream.Container, d.TranscodeStream.Codec)
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
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "opus"}},
	}

	if d := decideTranscode(info, track, "", nil); !d.CanDirectPlay {
		t.Fatalf("control: expected direct play without a forced profile, got %+v", d)
	}

	d := decideTranscode(info, track, "opus_rg", nil)
	if d.CanDirectPlay || !d.CanTranscode {
		t.Fatalf("expected forced transcode, got %+v", d)
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

	// 6 channels at 44100 so the negotiated stream must be downmixed and resampled, and the token must carry
	// both -- the user profile it names has to be able to apply them
	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 6, SampleRate: 44100, Bitrate: 900, BitDepth: 16}
	info := spec.ClientInfo{
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "opus", MaxAudioChannels: 2}},
	}

	d := decideTranscode(info, track, "", map[transcode.CodecName]string{transcode.CodecOpus.Name: "opus_192"})
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

func TestCodecFor(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		format string
		want   transcode.CodecName
	}{
		{"mp3", "mp3"},
		{"MP3", "mp3"},
		{"audio/mpeg", "mp3"},
		{"mpeg", "mp3"},
		{"opus", "opus"},
		{"ogg", "opus"}, // clients declare the container, gonic names the codec
		{"audio/ogg", "opus"},
		{"flac", "flac"},
		{"audio/flac", "flac"},
		{"wav", "pcm"},
	} {
		got, ok := codecFor(tc.format)
		if !ok || got.Name != tc.want {
			t.Errorf("codecFor(%q): got %q %v want %q", tc.format, got.Name, ok, tc.want)
		}
	}

	for _, format := range []string{"", "aac", "audio/aac", "raw", "audio"} {
		if got, ok := codecFor(format); ok {
			t.Errorf("codecFor(%q): got %q want no match", format, got.Name)
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
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "opus"}, {AudioCodec: "mp3"}},
	}

	d := decideTranscode(info, track, "opus_192", nil)
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
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "flac"}, {AudioCodec: "opus"}},
	}

	d := decideTranscode(info, track, "", nil)
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

// a 24 bit flac against a required "bitdepth <= 16" limit direct-play-fails on bit depth, then transcodes to
// 16 bit flac -- the conversion must survive into the token so getTranscodeStream applies -sample_fmt s16.
func TestDecideFlacBitDepthLimit(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 24}
	info := spec.ClientInfo{
		DirectPlayProfiles: []spec.DirectPlayProfile{{Containers: []string{"flac"}, AudioCodecs: []string{"flac"}}},
		CodecProfiles: []spec.CodecProfile{{
			Type: "AudioCodec", Name: "flac",
			Limitations: []spec.Limitation{{Name: "audioBitdepth", Comparison: "LessThanEqual", Values: []string{"16"}, Required: true}},
		}},
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "flac"}},
	}

	d := decideTranscode(info, track, "", nil)
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

// a lossy source never transcodes to a lossless target -- that's a pointless upscale, so flac is rejected
// and the next profile serves.
func TestDecideLossyToLosslessRejected(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.mp3", Codec: "mp3", Channels: 2, Bitrate: 192, SampleRate: 44100}
	info := spec.ClientInfo{
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "flac"}, {AudioCodec: "opus"}},
	}

	d := decideTranscode(info, track, "", nil)
	if !d.CanTranscode {
		t.Fatalf("expected transcode, got %+v", d)
	}
	if d.TranscodeStream.Codec != "opus" {
		t.Errorf("target codec: got %q want opus (flac must be rejected for a lossy source)", d.TranscodeStream.Codec)
	}
}

// a lossless target can't have its bitrate capped, so when the source exceeds a client cap the flac profile
// is rejected -- the source's bitrate is the only guess for the output's.
func TestDecideFlacRejectedByBitrateCap(t *testing.T) {
	t.Parallel()

	track := &db.Track{Filename: "song.flac", Codec: "flac", Channels: 2, Bitrate: 900, SampleRate: 44100, BitDepth: 16}
	info := spec.ClientInfo{
		MaxAudioBitRateBPS:  320_000,
		DirectPlayProfiles:  []spec.DirectPlayProfile{{Containers: []string{"flac"}, AudioCodecs: []string{"flac"}}},
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "flac"}, {AudioCodec: "opus"}},
	}

	d := decideTranscode(info, track, "", nil)
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
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "flac"}, {AudioCodec: "opus"}},
	}

	d := decideTranscode(info, track, "", nil)
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
		TranscodingProfiles: []spec.TranscodingProfile{{AudioCodec: "opus"}},
	}

	if d := decideTranscode(info, track, "mp3", nil); !d.CanDirectPlay {
		t.Errorf("expected fallback to direct play when the client can't accept the forced format, got %+v", d)
	}
}
