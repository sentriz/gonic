package ctrlsubsonic

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specidpaths"
	"go.senan.xyz/gonic/transcode"
)

func (c *Controller) ServeGetShares(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)

	shares, err := c.Shares.GetShares(user.ID, "")
	if err != nil {
		return spec.NewError(10, "failed get shares: %s", err)
	}

	sub := spec.NewResponse()
	sub.Shares = &spec.Shares{}
	for _, share := range shares {
		sh, err := shareRender(c, share, user)
		if err != nil {

			continue
		}
		sub.Shares.List = append(sub.Shares.List, sh)
	}

	return sub
}

func (c *Controller) ServeGetShare(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)
	shareID, err := params.GetFirst("id", "shareId")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}

	shares, err := c.Shares.GetShares(user.ID, shareID)
	if err != nil {
		return spec.NewError(10, "failed get share: %s", err)
	}

	if len(shares) == 0 {
		return spec.NewError(70, "nothing not found")
	}

	sh, err := shareRender(c, shares[0], user)
	if err != nil {
		return spec.NewError(0, "failed get share: %s", err)
	}

	sub := spec.NewResponse()
	sub.Share = sh
	return sub
}

func (c *Controller) ServeCreateShare(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)

	share := &db.Share{
		Title:     randKey(10),
		UserID:    user.ID,
		CreatedAt: time.Now(),
	}

	if val, err := params.Get("description"); err == nil {
		share.Description = val
	}

	if val, err := params.GetInt("expires"); err == nil {
		share.ExpiresAt = time.UnixMilli(int64(val))
	}

	if val, err := params.Get("secret"); err == nil {
		share.Secret = val
	}

	if val, err := params.GetBool("download"); err == nil {
		share.Download = val
	}

	share.Entries = []string{}
	ids := params.GetOrIDList("id", nil)
	for _, id := range ids {
		// TODO add id's check
		share.Entries = append(share.Entries, id.String())
	}

	if err := c.Shares.Save(share); err != nil {
		return spec.NewError(0, "save share: %v", err)
	}

	sub := spec.NewResponse()
	sh, err := shareRender(c, share, user)
	if err != nil {
		return spec.NewError(0, "error rendering share: %v", err)
	}
	sub.Shares = &spec.Shares{List: []*spec.Share{sh}}

	return sub
}

func (c *Controller) ServeUpdateShare(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)

	shareID, err := params.GetFirst("id", "shareId")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}

	shares, err := c.Shares.GetShares(user.ID, shareID)
	if len(shares) == 0 {
		return spec.NewError(70, "nothing not found")
	}
	if err != nil {
		return spec.NewError(0, "load share error")
	}

	share := shares[0]

	if val, err := params.Get("description"); err == nil {
		share.Description = val
	}

	if val, err := params.GetInt("expires"); err == nil {
		share.ExpiresAt = time.UnixMilli(int64(val))
	}

	if val, err := params.Get("secret"); err == nil {
		share.Secret = val
	}

	if val, err := params.GetBool("download"); err == nil {
		share.Download = val
	}

	addIDs := params.GetOrIDList("add", nil)
	for _, id := range addIDs {
		ids := id.String()
		// TODO add id's check
		founded := false
		for _, e := range share.Entries {
			if e == ids {
				founded = true
				break
			}
		}
		if founded {
			continue
		}
		share.Entries = append(share.Entries, ids)
	}

	removeIDs := params.GetOrIDList("remove", nil)
	for _, id := range removeIDs {
		ids := id.String()
		// TODO add id's check
		for i, e := range share.Entries {
			if e == ids {
				share.Entries[i] = share.Entries[len(share.Entries)-1]
				share.Entries = share.Entries[:len(share.Entries)-1]
				break
			}
		}
	}

	if err := c.Shares.Save(share); err != nil {
		return spec.NewError(0, "save share: %v", err)
	}

	return spec.NewResponse()
}

func (c *Controller) ServeDeleteShare(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)

	shareID, err := params.GetFirst("id", "shareId")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}

	if err := c.Shares.Delete(user.ID, shareID); err != nil {
		return spec.NewError(0, "delete playlist: %v", err)
	}

	return spec.NewResponse()
}

func (c *Controller) ServeGetSharePublic(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	shareID, err := params.GetFirst("shareId")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}

	shares, err := c.Shares.GetShares(0, shareID)
	if err != nil {
		return spec.NewError(10, "failed get share: %s", err)
	}

	if len(shares) == 0 {
		return spec.NewError(70, "nothing not found")
	}

	share := shares[0]
	if share.Secret != "" {
		secret, err := params.GetFirst("secret")
		if err != nil || secret != share.Secret {
			return spec.NewError(50, "auth required")
		}
	}

	share.LastVisitedAt = time.Now()
	share.VisitCount++
	go func() {
		if err := c.Shares.Save(share); err != nil {
			log.Printf("failed update share: %s", err)
		}
	}()

	user := &db.User{}
	if err := c.DB.Where("id=?", share.UserID).Find(user).Error; err != nil {
		return spec.NewError(0, "internal server error")
	}

	sh, err := shareRender(c, shares[0], user)
	if err != nil {
		return spec.NewError(0, "failed get share: %s", err)
	}

	sub := spec.NewResponse()
	sub.Share = sh
	return sub
}

func (c *Controller) ServeStreamSharePublic(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	shareID, err := params.GetFirst("shareId")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}

	shares, err := c.Shares.GetShares(0, shareID)
	if err != nil {
		return spec.NewError(10, "failed get share: %s", err)
	}

	if len(shares) == 0 {
		return spec.NewError(70, "nothing not found")
	}

	share := shares[0]
	if share.Secret != "" {
		secret, err := params.GetFirst("secret")
		if err != nil || secret != share.Secret {
			return spec.NewError(50, "auth required")
		}
	}

	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}

	found := false
	for _, e := range share.Entries {
		if e == id.String() {
			found = true
		}
	}

	if !found {
		return spec.NewError(10, "please provide a valid `id` parameter")
	}

	user := &db.User{}
	if err := c.DB.Where("id=?", share.UserID).Find(user).Error; err != nil {
		return spec.NewError(0, "internal server error")
	}

	file, err := specidpaths.Locate(c.DB, c.PodcastsPath, id)
	if err != nil {
		return spec.NewError(0, "error looking up id %s: %v", id, err)
	}

	audioFile, ok := file.(db.AudioFile)
	if !ok {
		return spec.NewError(0, "type of id does not contain audio")
	}

	if track, ok := audioFile.(*db.Track); ok && track.Album != nil {
		defer func() {
			if err := streamUpdateStats(c.DB, user.ID, track, time.Now()); err != nil {
				log.Printf("error updating track status: %v", err)
			}
		}()
	}

	if pe, ok := audioFile.(*db.PodcastEpisode); ok {
		defer func() {
			if err := streamUpdatePodcastEpisodeStats(c.DB, pe.ID); err != nil {
				log.Printf("error updating podcast episode status: %v", err)
			}
		}()
	}

	maxBitRate, _ := params.GetInt("maxBitRate")
	format, _ := params.Get("format")

	if format == "raw" || maxBitRate >= audioFile.AudioBitrate() {
		http.ServeFile(w, r, file.AbsPath())
		return nil
	}

	pref, err := streamGetTransPref(c.DB, user.ID, params.GetOr("c", ""))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(0, "couldn't find transcode preference: %v", err)
	}
	if pref == nil {
		http.ServeFile(w, r, file.AbsPath())
		return nil
	}

	profile, ok := transcode.UserProfiles[pref.Profile]
	if !ok {
		return spec.NewError(0, "unknown transcode user profile %q", pref.Profile)
	}
	if maxBitRate > 0 && int(profile.BitRate()) > maxBitRate {
		profile = transcode.WithBitrate(profile, transcode.BitRate(maxBitRate))
	}

	log.Printf("trancoding to %q with max bitrate %dk", profile.MIME(), profile.BitRate())

	w.Header().Set("Content-Type", profile.MIME())
	if err := c.Transcoder.Transcode(r.Context(), profile, file.AbsPath(), w); err != nil && !errors.Is(err, transcode.ErrFFmpegKilled) {
		return spec.NewError(0, "error transcoding: %v", err)
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func shareRender(c *Controller, share *db.Share, user *db.User) (*spec.Share, error) {
	ret := &spec.Share{
		ID:          share.Title,
		URL:         c.Shares.GetURL(share.Title),
		Description: share.Description,
		User:        user.Name,
		Created:     share.CreatedAt,
		Expires:     share.ExpiresAt,
		LastVisited: share.LastVisitedAt,
		VisitCount:  share.VisitCount,
		Protected:   share.Secret != "",
		Download:    share.Download,
		List:        []*spec.TrackChild{},
	}

	for _, entry := range share.Entries {
		sid, err := specid.New(entry)
		if err != nil {
			log.Printf("users %d share %s errored entry %s, %s", share.UserID, share.Title, entry, err)
			continue
		}

		var trch *spec.TrackChild
		switch sid.Type {
		case specid.Track:
			var track db.Track
			if err := c.DB.Where("id=?", sid.Value).Preload("Album").Preload("Album.TagArtist").Preload("TrackStar", "user_id=?", user.ID).Preload("TrackRating", "user_id=?", user.ID).Find(&track).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("load track by id: %w", err)
			}
			trch = spec.NewTCTrackByFolder(&track, track.Album)
		case specid.PodcastEpisode:
			var pe db.PodcastEpisode
			if err := c.DB.Where("id=?", sid.Value).Find(&pe).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("load podcast episode by id: %w", err)
			}
			var p db.Podcast
			if err := c.DB.Where("id=?", pe.PodcastID).Find(&p).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("load podcast by id: %w", err)
			}
			trch = spec.NewTCPodcastEpisode(&pe, &p)
		default:
			continue
		}

		// trch.TranscodedContentType = transcodeMIME
		// trch.TranscodedSuffix = transcodeSuffix

		ret.List = append(ret.List, trch)
	}

	return ret, nil
}

func randKey(l int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, l)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
