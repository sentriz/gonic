package spec

import (
	"go.senan.xyz/gonic/db"
)

func NewPodcastChannel(p *db.Podcast) *PodcastChannel {
	ret := &PodcastChannel{
		ID:               p.SID(),
		OriginalImageURL: p.ImageURL,
		Title:            p.Title,
		Description:      CleanExternalText(p.Description),
		URL:              p.URL,
		CoverArt:         p.SID(),
		Status:           "skipped",
	}
	for _, episode := range p.Episodes {
		specEpisode := NewPodcastEpisode(episode)
		ret.Episode = append(ret.Episode, specEpisode)
	}
	return ret
}

func NewPodcastEpisode(pe *db.PodcastEpisode) *PodcastEpisode {
	if pe == nil {
		return nil
	}
	r := &PodcastEpisode{
		ID:          pe.SID(),
		StreamID:    pe.SID(),
		ContentType: pe.MIME(),
		ChannelID:   pe.PodcastSID(),
		Title:       pe.Title,
		Description: CleanExternalText(pe.Description),
		Status:      string(pe.Status),
		CoverArt:    pe.PodcastSID(),
		PublishDate: *pe.PublishDate,
		Genre:       "Podcast",
		Duration:    pe.Length,
		Year:        pe.PublishDate.Year(),
		Suffix:      formatExt(pe.Ext()),
		BitRate:     pe.Bitrate,
		IsDir:       false,
		Size:        pe.Size,
	}
	if pe.Podcast != nil {
		r.Path = pe.AbsPath()
	}
	return r
}
