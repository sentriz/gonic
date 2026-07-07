package ctrlsubsonic

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specidpaths"
	"go.senan.xyz/gonic/tags"
	"go.senan.xyz/gonic/transcode"
)

// the opensubsonic "transcoding" extension
// https://opensubsonic.netlify.app/docs/extensions/transcoding/

func (c *Controller) ServeGetTranscodeDecision(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)

	id, err := params.GetID("mediaId")
	if err != nil {
		return spec.NewError(10, "please provide a `mediaId` parameter")
	}

	mediaType, err := params.Get("mediaType")
	if err != nil {
		return spec.NewError(10, "please provide a `mediaType` parameter")
	}

	audioFile, err := specidpaths.LocateMedia(c.dbc, mediaType, id)
	if err != nil {
		return spec.NewError(70, "couldn't find a song or podcast episode with that id")
	}

	var info spec.ClientInfo
	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		if errors.Is(err, io.EOF) {
			return spec.NewError(10, "please provide a client info body")
		}
		return spec.NewError(0, "decode client info body: %v", err)
	}

	if err := backfillAudioProps(c.dbc, c.tagReader, audioFile); err != nil {
		log.Printf("backfilling audio props for %s: %v", id, err)
	}

	user := r.Context().Value(CtxUser).(*db.User)
	client, _ := params.Get("c")

	forcedProfile, _, err := streamGetTranscodePreference(c.dbc, user.ID, client)
	if err != nil {
		return spec.NewError(0, "check transcode preference: %v", err)
	}

	prefs, err := formatPrefs(c.dbc, user.ID)
	if err != nil {
		return spec.NewError(0, "check transcode format preferences: %v", err)
	}

	sub := spec.NewResponse()
	sub.TranscodeDecision = decideTranscode(info, audioFile, forcedProfile, prefs)

	return sub
}

func (c *Controller) ServeGetTranscodeStream(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)

	id, err := params.GetID("mediaId")
	if err != nil {
		return spec.NewError(10, "please provide a `mediaId` parameter")
	}

	mediaType, err := params.Get("mediaType")
	if err != nil {
		return spec.NewError(10, "please provide a `mediaType` parameter")
	}

	audio, err := specidpaths.LocateMedia(c.dbc, mediaType, id)
	if err != nil {
		return spec.NewError(70, "couldn't find a song or podcast episode with that id")
	}

	raw, err := params.Get("transcodeParams")
	if err != nil {
		return spec.NewError(10, "please provide a `transcodeParams` parameter")
	}
	tp, err := decodeTranscodeParams(raw)
	if err != nil {
		return spec.NewError(0, "invalid transcodeParams: %v", err)
	}
	if tp.MediaID != id.String() {
		return spec.NewError(0, "transcodeParams was issued for a different media id")
	}

	client, _ := params.Get("c")

	if tp.DirectPlay {
		log.Printf("serving raw file %q for user %q client %q", audio.AudioFilename(), user.Name, client)
		http.ServeFile(w, r, audio.AbsPath()) //nolint:gosec // path is from db, populated by scanner
		return nil
	}

	profile, ok := transcode.DefaultProfiles[tp.Codec]
	if tp.Profile != "" {
		profile, ok = transcode.UserProfiles[tp.Profile]
	}
	if !ok {
		return spec.NewError(0, "unsupported profile in transcodeParams")
	}
	profile = transcode.WithBitrate(profile, transcode.BitRate(tp.BitRate))
	profile = transcode.WithChannels(profile, tp.Channels)
	profile = transcode.WithSampleRate(profile, tp.SampleRate)
	profile = transcode.WithBitDepth(profile, tp.BitDepth)
	profile = transcode.WithSeek(profile, time.Second*time.Duration(params.GetOrInt("offset", 0)))

	w.Header().Set("Content-Type", profile.MIME())

	if ct, ok := c.transcoder.(*transcode.CachingTranscoder); ok {
		path, release, err := ct.CachedPath(profile, audio.AbsPath())
		if err != nil {
			return spec.NewError(0, "check transcode cache: %v", err)
		}
		if path != "" {
			defer release()
			log.Printf("serving cached transcode of %q to %q at bitrate %d for user %q client %q", audio.AudioFilename(), profile.MIME(), profile.BitRate(), user.Name, client)
			http.ServeFile(w, r, path) //nolint:gosec // path is a cache filename derived from an md5 key
			return nil
		}
	}

	log.Printf("transcoding %q to %q at bitrate %d for user %q client %q", audio.AudioFilename(), profile.MIME(), profile.BitRate(), user.Name, client)

	if err := c.transcoder.Transcode(r.Context(), profile, audio.AbsPath(), w); err != nil && !errors.Is(err, transcode.ErrFFmpegKilled) && !errors.Is(err, context.Canceled) {
		return spec.NewError(0, "error transcoding: %v", err)
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func backfillAudioProps(dbc *db.DB, tagReader tags.Reader, audio db.AudioFile) error {
	if audio.AudioCodec() != "" {
		return nil
	}
	props, _, err := tagReader.Read(audio.AbsPath())
	if err != nil {
		return fmt.Errorf("read tags: %w", err)
	}
	columns := map[string]any{
		"codec":       props.Codec,
		"channels":    int(props.Channels),
		"sample_rate": int(props.SampleRate),
		"bit_depth":   int(props.BitDepth),
	}
	if err := dbc.Model(audio).UpdateColumns(columns).Error; err != nil {
		return fmt.Errorf("update audio properties: %w", err)
	}
	return nil
}

func formatPrefs(dbc *db.DB, userID int) (map[transcode.CodecName]string, error) {
	var prefs []*db.TranscodeFormatPreference
	if err := dbc.Where("user_id=?", userID).Order("created_at").Find(&prefs).Error; err != nil {
		return nil, fmt.Errorf("find format prefs: %w", err)
	}
	byCodec := map[transcode.CodecName]string{}
	for _, fp := range prefs {
		p, ok := transcode.UserProfiles[fp.Profile]
		if !ok {
			return nil, fmt.Errorf("unknown transcode user profile %q", fp.Profile)
		}
		if name := p.Codec().Name; byCodec[name] == "" {
			byCodec[name] = fp.Profile
		}
	}
	return byCodec, nil
}

func decideTranscode(info spec.ClientInfo, audio db.AudioFile, forced string, formatPrefs map[transcode.CodecName]string) *spec.TranscodeDecision {
	src := spec.StreamDetails{
		Protocol:        protocolHTTP,
		Container:       strings.TrimPrefix(audio.Ext(), "."),
		Codec:           audio.AudioCodec(),
		AudioBitRateBPS: audio.AudioBitrate() * 1000, // db is kbps
		AudioChannels:   audio.AudioChannels(),
		AudioSampleRate: audio.AudioSampleRate(),
		AudioBitDepth:   audio.AudioBitDepth(),
	}

	// a transcode pref is policy, not capability, so it must never be the reason a track won't play. if
	// narrowing to its format yields nothing, negotiate as though it weren't set
	pref, forcing := transcode.UserProfiles[forced]
	i := -1
	if forcing {
		i = slices.IndexFunc(info.TranscodingProfiles, func(p spec.TranscodingProfile) bool {
			codec, ok := codecFor(cmp.Or(p.AudioCodec, p.Container))
			return ok && codec.Name == pref.Codec().Name
		})
	}
	if i >= 0 {
		narrowed := info
		narrowed.DirectPlayProfiles = nil
		narrowed.TranscodingProfiles = info.TranscodingProfiles[i : i+1]
		if bps := int(pref.BitRate()) * 1000; bps > 0 && (narrowed.MaxTranscodingAudioBitRateBPS == 0 || bps < narrowed.MaxTranscodingAudioBitRateBPS) {
			narrowed.MaxTranscodingAudioBitRateBPS = bps
		}
		if d := negotiate(narrowed, src, audio, map[transcode.CodecName]string{pref.Codec().Name: forced}); d.CanTranscode {
			d.TranscodeReason = append([]string{reasonTranscodePreference}, d.TranscodeReason...)
			return d
		}
	}

	return negotiate(info, src, audio, formatPrefs)
}

func negotiate(info spec.ClientInfo, src spec.StreamDetails, audio db.AudioFile, formatPrefs map[transcode.CodecName]string) *spec.TranscodeDecision {
	d := &spec.TranscodeDecision{SourceStream: &src}
	mediaID := audio.SID().String()

	if info.MaxAudioBitRateBPS > 0 && src.AudioBitRateBPS > info.MaxAudioBitRateBPS {
		d.TranscodeReason = []string{reasonAudioBitrate}
	} else {
		matching := matchingProfiles(info.CodecProfiles, src.Codec)
		for _, p := range info.DirectPlayProfiles {
			reason := directPlayReason(src, p, matching)
			if reason == "" {
				d.CanDirectPlay = true
				d.TranscodeParams = encodeTranscodeParams(transcodeParams{MediaID: mediaID, DirectPlay: true})
				return d
			}
			if !slices.Contains(d.TranscodeReason, reason) {
				d.TranscodeReason = append(d.TranscodeReason, reason)
			}
		}
	}

	for _, p := range info.TranscodingProfiles {
		ts, codec, ok := computeTranscode(src, p, info, formatPrefs)
		if !ok {
			continue
		}
		changed := func(to, from int) int {
			if to == from {
				return 0
			}
			return to
		}
		d.CanTranscode = true
		d.TranscodeStream = &ts
		d.TranscodeParams = encodeTranscodeParams(transcodeParams{
			MediaID:    mediaID,
			Profile:    formatPrefs[codec.Name],
			Codec:      codec.Name,
			BitRate:    ts.AudioBitRateBPS / 1000,
			Channels:   changed(ts.AudioChannels, src.AudioChannels),
			SampleRate: changed(ts.AudioSampleRate, src.AudioSampleRate),
			BitDepth:   changed(ts.AudioBitDepth, src.AudioBitDepth),
		})
		return d
	}

	d.ErrorReason = "no compatible playback profile found"
	return d
}

func directPlayReason(src spec.StreamDetails, p spec.DirectPlayProfile, matching []spec.CodecProfile) string {
	switch {
	case len(p.Protocols) > 0 && !containsFold(p.Protocols, protocolHTTP):
		return reasonProtocol
	case len(p.Containers) > 0 && !containsFormat(p.Containers, src.Container):
		return reasonContainer
	case len(p.AudioCodecs) > 0 && !containsFold(p.AudioCodecs, src.Codec):
		return reasonAudioCodec
	case p.MaxAudioChannels > 0 && src.AudioChannels > p.MaxAudioChannels:
		return reasonAudioChannels
	}
	return limitationReason(src, matching)
}

func computeTranscode(src spec.StreamDetails, p spec.TranscodingProfile, info spec.ClientInfo, formatPrefs map[transcode.CodecName]string) (spec.StreamDetails, transcode.Codec, bool) {
	if p.Protocol != "" && !strings.EqualFold(p.Protocol, protocolHTTP) {
		return spec.StreamDetails{}, transcode.Codec{}, false
	}

	codec, ok := codecFor(cmp.Or(p.AudioCodec, p.Container))
	if !ok {
		return spec.StreamDetails{}, transcode.Codec{}, false
	}
	base, ok := transcode.DefaultProfiles[codec.Name]
	if !ok {
		return spec.StreamDetails{}, transcode.Codec{}, false // a codec gonic never offers, e.g. pcm
	}
	if name := formatPrefs[codec.Name]; name != "" {
		base = transcode.UserProfiles[name]
	}

	ts := spec.StreamDetails{
		Protocol:        protocolHTTP,
		Container:       cmp.Or(strings.ToLower(p.Container), string(codec.Name)),
		Codec:           string(codec.Name),
		AudioBitRateBPS: src.AudioBitRateBPS,
		AudioChannels:   src.AudioChannels,
		AudioSampleRate: src.AudioSampleRate,
	}
	if codec.Lossless {
		ts.AudioBitDepth = src.AudioBitDepth
	} else if src.AudioBitDepth > 0 || src.AudioBitRateBPS == 0 {
		// only lossless sources have a bit depth scanned, and their bitrate means nothing to a lossy target
		ts.AudioBitRateBPS = cmp.Or(info.MaxTranscodingAudioBitRateBPS, info.MaxAudioBitRateBPS, int(base.BitRate())*1000)
	}
	for _, capBPS := range []int{info.MaxTranscodingAudioBitRateBPS, info.MaxAudioBitRateBPS} {
		if capBPS > 0 {
			ts.AudioBitRateBPS = min(ts.AudioBitRateBPS, capBPS)
		}
	}
	if p.MaxAudioChannels > 0 {
		ts.AudioChannels = min(ts.AudioChannels, p.MaxAudioChannels)
	}
	ts.AudioChannels = min(ts.AudioChannels, codec.MaxChannels)

	matching := matchingProfiles(info.CodecProfiles, ts.Codec)
	for _, cp := range matching {
		for _, lim := range cp.Limitations {
			if field, _, ok := streamField(&ts, lim.Name); ok && !adjust(lim, field) {
				return spec.StreamDetails{}, transcode.Codec{}, false
			}
		}
	}

	if codec.Lossless {
		// no depth means a lossy source, a pointless upscale. and a lossless bitrate can't be capped, so the
		// source's has to fit
		ts.AudioBitDepth = transcode.NearestBitDepth(codec, ts.AudioBitDepth)
		if ts.AudioBitDepth == 0 || ts.AudioBitRateBPS != src.AudioBitRateBPS {
			return spec.StreamDetails{}, transcode.Codec{}, false
		}
		ts.AudioBitRateBPS = 0
	}
	ts.AudioSampleRate = transcode.NearestSampleRate(codec, ts.AudioSampleRate)

	// snapping to what the codec can encode (opus can't emit 44100) may break a required limitation after all
	if limitationReason(ts, matching) != "" {
		return spec.StreamDetails{}, transcode.Codec{}, false
	}
	return ts, codec, true
}

func limitationReason(s spec.StreamDetails, matching []spec.CodecProfile) string {
	for _, cp := range matching {
		for _, lim := range cp.Limitations {
			if field, reason, ok := streamField(&s, lim.Name); ok && lim.Required && !satisfies(lim, *field) {
				return reason
			}
		}
	}
	return ""
}

const protocolHTTP = "http"

// transcodeReason strings as commonly emitted by other opensubsonic servers
const (
	reasonProtocol        = "protocol not supported"
	reasonContainer       = "container not supported"
	reasonAudioCodec      = "audio codec not supported"
	reasonAudioChannels   = "audio channels not supported"
	reasonAudioBitrate    = "audio bitrate not supported"
	reasonAudioSamplerate = "audio samplerate not supported"
	reasonAudioBitdepth   = "audio bitdepth not supported"

	// gonic's own, emitted when a transcode pref denies direct play
	reasonTranscodePreference = "server transcode preference"
)

func matchingProfiles(codecProfiles []spec.CodecProfile, codec string) []spec.CodecProfile {
	var out []spec.CodecProfile
	for _, cp := range codecProfiles {
		if strings.EqualFold(cp.Type, spec.CodecProfileTypeAudio) && strings.EqualFold(cp.Name, codec) {
			out = append(out, cp)
		}
	}
	return out
}

// gonic has no audioProfile data, so that limitation is intentionally unsupported
func streamField(s *spec.StreamDetails, name string) (field *int, reason string, ok bool) {
	switch name {
	case spec.LimitationAudioChannels:
		return &s.AudioChannels, reasonAudioChannels, true
	case spec.LimitationAudioSamplerate:
		return &s.AudioSampleRate, reasonAudioSamplerate, true
	case spec.LimitationAudioBitrate:
		return &s.AudioBitRateBPS, reasonAudioBitrate, true
	case spec.LimitationAudioBitDepth:
		return &s.AudioBitDepth, reasonAudioBitdepth, true
	}
	return nil, "", false
}

func satisfies(lim spec.Limitation, value int) bool {
	v := value
	return adjust(lim, &v) && v == value
}

// adjust moves v down to meet the limitation, reporting false when it can't without upscaling
func adjust(lim spec.Limitation, v *int) bool {
	var values []int
	for _, s := range lim.Values {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			values = append(values, n)
		}
	}
	if len(values) == 0 || *v == 0 {
		return true // unknown or unscanned properties never block
	}
	switch lim.Comparison {
	case spec.ComparisonLessThanEqual:
		*v = min(*v, values[0])
	case spec.ComparisonGreaterThanEqual:
		return *v >= values[0]
	case spec.ComparisonEquals:
		if slices.Contains(values, *v) {
			return true
		}
		var below int
		for _, n := range values {
			if n < *v && n > below {
				below = n
			}
		}
		*v = below
		return below > 0
	case spec.ComparisonNotEquals:
		return !slices.Contains(values, *v)
	}
	return true
}

// transcodeParams is the stateless getTranscodeStream token, bound to the media it was decided for. Profile,
// when set, names the UserProfiles entry to use instead of the base codec profile (e.g. a replaygain variant)
type transcodeParams struct {
	MediaID    string              `json:"mid,omitempty"`
	DirectPlay bool                `json:"dp,omitempty"`
	Profile    string              `json:"p,omitempty"`
	Codec      transcode.CodecName `json:"c,omitempty"`
	BitRate    int                 `json:"b,omitempty"`
	Channels   int                 `json:"ch,omitempty"`
	SampleRate int                 `json:"sr,omitempty"`
	BitDepth   int                 `json:"bd,omitempty"`
}

func encodeTranscodeParams(tp transcodeParams) string {
	b, err := json.Marshal(tp)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeTranscodeParams(raw string) (transcodeParams, error) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return transcodeParams{}, err
	}
	var tp transcodeParams
	if err := json.Unmarshal(b, &tp); err != nil {
		return transcodeParams{}, err
	}
	return tp, nil
}

func containsFold(values []string, value string) bool {
	for _, v := range values {
		if strings.EqualFold(v, value) {
			return true
		}
	}
	return false
}

// codecFor resolves a format as clients spell it to the codec gonic would encode it with, so a client
// asking for "ogg" gets opus
func codecFor(format string) (transcode.Codec, bool) {
	format = strings.ToLower(format)
	for _, c := range transcode.Codecs {
		if slices.Contains(codecAliases(c), format) {
			return c, true
		}
	}
	return transcode.Codec{}, false
}

// the names a client might use for a codec: its own name, the file suffix, or the MIME type with and
// without its "audio/" prefix, which is where "ogg" for opus comes from
func codecAliases(c transcode.Codec) []string {
	return []string{string(c.Name), c.Suffix, c.MIME, strings.TrimPrefix(c.MIME, "audio/")}
}

// matches by extension or MIME type, so a client declaring container "ogg" matches gonic's ".opus" files
func containsFormat(values []string, ext string) bool {
	extMIME := mime.TypeByExtension("." + strings.ToLower(ext))
	for _, v := range values {
		if strings.EqualFold(v, ext) {
			return true
		}
		if extMIME != "" && mime.TypeByExtension("."+strings.ToLower(v)) == extMIME {
			return true
		}
	}
	return false
}
