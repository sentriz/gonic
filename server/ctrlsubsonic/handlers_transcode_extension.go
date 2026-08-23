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
	if tp.BitRate > 0 {
		profile = transcode.WithBitrate(profile, transcode.BitRate(tp.BitRate))
	}
	if tp.Channels > 0 {
		profile = transcode.WithChannels(profile, tp.Channels)
	}
	if tp.SampleRate > 0 {
		profile = transcode.WithSampleRate(profile, tp.SampleRate)
	}
	if tp.BitDepth > 0 {
		profile = transcode.WithBitDepth(profile, tp.BitDepth)
	}
	if offset := params.GetOrInt("offset", 0); offset > 0 {
		profile = transcode.WithSeek(profile, time.Second*time.Duration(offset))
	}

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
	switch a := audio.(type) {
	case *db.Track:
		a.Codec, a.Channels, a.SampleRate, a.BitDepth = props.Codec, int(props.Channels), int(props.SampleRate), int(props.BitDepth)
		return dbc.Save(a).Error
	case *db.PodcastEpisode:
		a.Codec, a.Channels, a.SampleRate, a.BitDepth = props.Codec, int(props.Channels), int(props.SampleRate), int(props.BitDepth)
		return dbc.Save(a).Error
	}
	return nil
}

func formatPrefs(dbc *db.DB, userID int) (map[transcode.CodecName]string, error) {
	var prefs []*db.TranscodeFormatPreference
	if err := dbc.Where("user_id=?", userID).Order("created_at").Find(&prefs).Error; err != nil {
		return nil, fmt.Errorf("find format prefs: %w", err)
	}
	byEncoder := map[transcode.CodecName]string{}
	for _, fp := range prefs {
		p, ok := transcode.UserProfiles[fp.Profile]
		if !ok {
			return nil, fmt.Errorf("unknown transcode user profile %q", fp.Profile)
		}
		if name := p.Codec().Name; byEncoder[name] == "" {
			byEncoder[name] = fp.Profile
		}
	}
	return byEncoder, nil
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

	// a transcode pref denies direct play and narrows negotiation to its format. it's a policy, not a
	// capability, so it must never be the reason a track won't play -- if it yields nothing, negotiate again
	// as though it weren't set
	if forced != "" {
		if narrowed, ok := narrowToPref(info, transcode.UserProfiles[forced]); ok {
			if d := negotiate(narrowed, src, audio, nil, forced); d.CanTranscode {
				d.TranscodeReason = append([]string{reasonTranscodePreference}, d.TranscodeReason...)
				return d
			}
		}
	}

	return negotiate(info, src, audio, formatPrefs, "")
}

func narrowToPref(info spec.ClientInfo, pref transcode.Profile) (spec.ClientInfo, bool) {
	i := slices.IndexFunc(info.TranscodingProfiles, func(p spec.TranscodingProfile) bool {
		codec, ok := codecFor(cmp.Or(p.AudioCodec, p.Container))
		return ok && codec.Name == pref.Codec().Name
	})
	if i < 0 {
		return info, false // the client can't accept the pref's format
	}
	info.DirectPlayProfiles = nil
	info.TranscodingProfiles = info.TranscodingProfiles[i : i+1]
	if bps := int(pref.BitRate()) * 1000; bps > 0 && (info.MaxTranscodingAudioBitRateBPS == 0 || bps < info.MaxTranscodingAudioBitRateBPS) {
		info.MaxTranscodingAudioBitRateBPS = bps
	}
	return info, true
}

func negotiate(info spec.ClientInfo, src spec.StreamDetails, audio db.AudioFile, formatPrefs map[transcode.CodecName]string, forced string) *spec.TranscodeDecision {
	var d spec.TranscodeDecision
	d.SourceStream = &src

	var reasons []string
	addReason := func(r string) {
		if !slices.Contains(reasons, r) {
			reasons = append(reasons, r)
		}
	}

	matching := matchingProfiles(info.CodecProfiles, src.Codec)

	if info.MaxAudioBitRateBPS > 0 && src.AudioBitRateBPS > info.MaxAudioBitRateBPS {
		addReason(reasonAudioBitrate)
	} else {
		for _, p := range info.DirectPlayProfiles {
			reason := directPlayReason(src, p, matching)
			if reason == "" {
				d.CanDirectPlay = true
				d.TranscodeParams = encodeTranscodeParams(transcodeParams{MediaID: audio.SID().String(), DirectPlay: true})
				return &d
			}
			addReason(reason)
		}
	}

	d.TranscodeReason = reasons

	for _, p := range info.TranscodingProfiles {
		if ts, enc, ok := computeTranscode(src, p, info, formatPrefs); ok {
			d.CanTranscode = true
			tp := transcodeParams{MediaID: audio.SID().String(), Profile: cmp.Or(forced, formatPrefs[enc.Name]), Codec: enc.Name, BitRate: bpsToKbps(ts.AudioBitRateBPS)}
			if ts.AudioChannels != src.AudioChannels {
				tp.Channels = ts.AudioChannels
			}
			if ts.AudioSampleRate != src.AudioSampleRate {
				tp.SampleRate = ts.AudioSampleRate
			}
			if ts.AudioBitDepth > 0 && ts.AudioBitDepth != src.AudioBitDepth {
				tp.BitDepth = ts.AudioBitDepth
			}
			d.TranscodeParams = encodeTranscodeParams(tp)
			d.TranscodeStream = &ts
			return &d
		}
	}

	d.ErrorReason = "no compatible playback profile found"
	return &d
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
	for _, cp := range matching {
		for _, lim := range cp.Limitations {
			field, reason, ok := streamField(&src, lim.Name)
			if !lim.Required || !ok {
				continue
			}
			if !satisfies(lim, *field) {
				return reason
			}
		}
	}
	return ""
}

func computeTranscode(src spec.StreamDetails, p spec.TranscodingProfile, info spec.ClientInfo, formatPrefs map[transcode.CodecName]string) (spec.StreamDetails, transcode.Codec, bool) {
	if p.Protocol != "" && !strings.EqualFold(p.Protocol, protocolHTTP) {
		return spec.StreamDetails{}, transcode.Codec{}, false
	}

	enc, ok := codecFor(cmp.Or(p.AudioCodec, p.Container))
	if !ok {
		return spec.StreamDetails{}, transcode.Codec{}, false
	}
	base, ok := transcode.DefaultProfiles[enc.Name]
	if !ok {
		return spec.StreamDetails{}, transcode.Codec{}, false // an encoder gonic never offers, e.g. pcm
	}

	if name := formatPrefs[enc.Name]; name != "" {
		base = transcode.UserProfiles[name]
	}

	// a lossless target's bitrate can't be capped, and the source's is the only guess for the output's, so
	// reject the profile when the source exceeds a client cap
	losslessTarget := enc.Lossless
	if losslessTarget {
		if src.AudioBitDepth == 0 {
			return spec.StreamDetails{}, transcode.Codec{}, false // lossy -> lossless is a pointless upscale
		}
		for _, capBPS := range []int{info.MaxTranscodingAudioBitRateBPS, info.MaxAudioBitRateBPS} {
			if capBPS > 0 && src.AudioBitRateBPS > capBPS {
				return spec.StreamDetails{}, transcode.Codec{}, false
			}
		}
	}

	ts := spec.StreamDetails{
		Protocol:        protocolHTTP,
		Container:       cmp.Or(strings.ToLower(p.Container), string(enc.Name)),
		Codec:           string(enc.Name),
		AudioBitRateBPS: targetBitRateBPS(src, base, info),
		AudioChannels:   src.AudioChannels,
		AudioSampleRate: src.AudioSampleRate,
	}
	if losslessTarget {
		ts.AudioBitDepth = src.AudioBitDepth
	}
	if p.MaxAudioChannels > 0 && ts.AudioChannels > p.MaxAudioChannels {
		ts.AudioChannels = p.MaxAudioChannels
	}
	if enc.MaxChannels > 0 && ts.AudioChannels > enc.MaxChannels {
		ts.AudioChannels = enc.MaxChannels
	}

	matching := matchingProfiles(info.CodecProfiles, ts.Codec)
	if !applyLimitations(&ts, src, matching, losslessTarget) {
		return spec.StreamDetails{}, transcode.Codec{}, false
	}

	if losslessTarget && ts.AudioBitDepth != src.AudioBitDepth {
		// an adjusted depth has to snap to one the codec stores, and may then break the limitation after all
		ts.AudioBitDepth = transcode.NearestBitDepth(enc, ts.AudioBitDepth)
		if ts.AudioBitDepth == 0 {
			return spec.StreamDetails{}, transcode.Codec{}, false
		}
		if !requiredSatisfied(matching, spec.LimitationAudioBitDepth, ts.AudioBitDepth) {
			return spec.StreamDetails{}, transcode.Codec{}, false
		}
	}

	rate, ok := resolveSampleRate(enc, src.AudioSampleRate, matching)
	if !ok {
		return spec.StreamDetails{}, transcode.Codec{}, false
	}
	if rate != 0 {
		ts.AudioSampleRate = rate
	}
	return ts, enc, true
}

func applyLimitations(ts *spec.StreamDetails, src spec.StreamDetails, matching []spec.CodecProfile, losslessTarget bool) bool {
	for _, cp := range matching {
		for _, lim := range cp.Limitations {
			switch lim.Name {
			case spec.LimitationAudioSamplerate:
				continue
			case spec.LimitationAudioBitrate:
				if losslessTarget {
					if !satisfies(lim, src.AudioBitRateBPS) {
						return false
					}
					continue
				}
			case spec.LimitationAudioBitDepth:
				if !losslessTarget {
					continue
				}
			}
			if field, _, ok := streamField(ts, lim.Name); ok && !adjust(lim, field) {
				return false
			}
		}
	}
	return true
}

// a lossy source keeps its own bitrate; a lossless one (only those have a bit depth scanned) gets the
// client's cap or the profile's default. everything is then capped down by the client's limits
func targetBitRateBPS(src spec.StreamDetails, base transcode.Profile, info spec.ClientInfo) int {
	if base.BitRate() == 0 {
		return 0
	}
	bitRate := src.AudioBitRateBPS
	if src.AudioBitDepth > 0 || bitRate == 0 {
		bitRate = cmp.Or(info.MaxTranscodingAudioBitRateBPS, info.MaxAudioBitRateBPS, int(base.BitRate())*1000)
	}
	for _, capBPS := range []int{info.MaxTranscodingAudioBitRateBPS, info.MaxAudioBitRateBPS} {
		if capBPS > 0 && capBPS < bitRate {
			bitRate = capBPS
		}
	}
	return bitRate
}

// the desired rate is snapped to the encoder's discrete rates (opus can't emit 44100, so it becomes 48000),
// re-checking required limitations against the actual output rate. 0 means keep the source's rate
func resolveSampleRate(enc transcode.Codec, src int, matching []spec.CodecProfile) (int, bool) {
	rate := src
	for _, cp := range matching {
		for _, lim := range cp.Limitations {
			if lim.Name != spec.LimitationAudioSamplerate {
				continue
			}
			if !adjust(lim, &rate) {
				return 0, false
			}
		}
	}
	if rate == 0 {
		return 0, true
	}

	rate = transcode.NearestSampleRate(enc, rate)
	if !requiredSatisfied(matching, spec.LimitationAudioSamplerate, rate) {
		return 0, false
	}

	if rate == src {
		return 0, true
	}
	return rate, true
}

func requiredSatisfied(matching []spec.CodecProfile, name string, value int) bool {
	for _, cp := range matching {
		for _, lim := range cp.Limitations {
			if lim.Name == name && lim.Required && !satisfies(lim, value) {
				return false
			}
		}
	}
	return true
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
	if len(lim.Values) == 0 || value == 0 {
		return true // unknown or unscanned properties never block
	}
	switch lim.Comparison {
	case spec.ComparisonLessThanEqual:
		limit, ok := parseNonNeg(lim.Values[0])
		return !ok || value <= limit
	case spec.ComparisonGreaterThanEqual:
		limit, ok := parseNonNeg(lim.Values[0])
		return !ok || value >= limit
	case spec.ComparisonEquals:
		return anyEquals(lim.Values, value)
	case spec.ComparisonNotEquals:
		return !anyEquals(lim.Values, value)
	}
	return true
}

func adjust(lim spec.Limitation, v *int) bool {
	if len(lim.Values) == 0 || *v == 0 {
		return true // unknown or unscanned properties never block
	}
	switch lim.Comparison {
	case spec.ComparisonLessThanEqual:
		if limit, ok := parseNonNeg(lim.Values[0]); ok && *v > limit {
			*v = limit
		}
	case spec.ComparisonGreaterThanEqual:
		if limit, ok := parseNonNeg(lim.Values[0]); ok && *v < limit {
			return false // can't upscale
		}
	case spec.ComparisonEquals:
		if anyEquals(lim.Values, *v) {
			return true
		}
		if closest, ok := closestBelow(lim.Values, *v); ok {
			*v = closest
			return true
		}
		return false
	case spec.ComparisonNotEquals:
		return !anyEquals(lim.Values, *v)
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

// floors, so the token's kbps never rounds up past the client's declared cap
func bpsToKbps(bps int) int {
	if bps <= 0 {
		return 0
	}
	return max(bps/1000, 1) // 0 would mean "the profile's default", undoing a cap rather than applying it
}

func containsFold(values []string, value string) bool {
	for _, v := range values {
		if strings.EqualFold(v, value) {
			return true
		}
	}
	return false
}

// codecFor resolves a format as clients spell it -- codec name, file suffix, or MIME type -- to the codec
// gonic would encode it with, so a client asking for "ogg" gets opus
func codecFor(format string) (transcode.Codec, bool) {
	format = strings.ToLower(format)
	for _, c := range transcode.Codecs {
		switch format {
		case string(c.Name), c.Suffix, c.MIME, strings.TrimPrefix(c.MIME, "audio/"):
			return c, true
		}
	}
	return transcode.Codec{}, false
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

func anyEquals(values []string, value int) bool {
	for _, v := range values {
		if limit, ok := parseNonNeg(v); ok && value == limit {
			return true
		}
	}
	return false
}

func closestBelow(values []string, value int) (int, bool) {
	closest, found := 0, false
	for _, v := range values {
		if limit, ok := parseNonNeg(v); ok && limit < value && (!found || limit > closest) {
			closest, found = limit, true
		}
	}
	return closest, found
}

func parseNonNeg(value string) (int, bool) {
	v, err := strconv.Atoi(value)
	if err != nil || v < 0 {
		return 0, false
	}
	return v, true
}
