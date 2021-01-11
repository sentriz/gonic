package spec

import "go.senan.xyz/gonic/server/db"

func NewPodcastChannel(p *db.Podcast) *PodcastChannel {
	ret := &PodcastChannel{
		ID:               p.SID(),
		OriginalImageURL: p.ImageURL,
		Title:            p.Title,
		Description:      p.Description,
		URL:              p.URL,
		CoverArt:         p.SID(),
		Status:           "skipped",
	}
	for _, episode := range p.Episodes {
		specEpisode := NewPodcastEpisode(p, episode)
		ret.Episode = append(ret.Episode, specEpisode)
	}
	return ret
}

func NewPodcastEpisode(p *db.Podcast, e *db.PodcastEpisode) *PodcastEpisode {
	if e == nil {
		return nil
	}
	return &PodcastEpisode{
		ID:          e.SID(),
		StreamID:    e.SID(),
		ContentType: e.MIME(),
		ChannelID:   p.SID(),
		Title:       e.Title,
		Description: e.Description,
		Status:      e.Status,
		CoverArt:    p.SID(),
		PublishDate: *e.PublishDate,
		Genre:       "Podcast",
		Duration:    e.Length,
		Year:        e.PublishDate.Year(),
		Suffix:      e.Ext(),
		BitRate:     e.Bitrate,
		IsDir:       false,
		Path:        e.Path,
		Size:        e.Size,
	}
}
