package spec

import (
	"jaytaylor.com/html2text"
	"go.senan.xyz/gonic/db"
)

func NewPodcastChannel(p *db.Podcast) *PodcastChannel {
	desc, err := html2text.FromString(p.Description, html2text.Options{TextOnly: true})
	if (err != nil) {
		desc = ""
	}
	ret := &PodcastChannel{
		ID:               p.SID(),
		OriginalImageURL: p.ImageURL,
		Title:            p.Title,
		Description:      desc,
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

func NewPodcastEpisode(e *db.PodcastEpisode) *PodcastEpisode {
	if e == nil {
		return nil
	}
	desc, err := html2text.FromString(e.Description, html2text.Options{TextOnly: true})
	if (err != nil) {
		desc = ""
	}
	return &PodcastEpisode{
		ID:          e.SID(),
		StreamID:    e.SID(),
		ContentType: e.MIME(),
		ChannelID:   e.PodcastSID(),
		Title:       e.Title,
		Description: desc,
		Status:      string(e.Status),
		CoverArt:    e.PodcastSID(),
		PublishDate: *e.PublishDate,
		Genre:       "Podcast",
		Duration:    e.Length,
		Year:        e.PublishDate.Year(),
		Suffix:      formatExt(e.Ext()),
		BitRate:     e.Bitrate,
		IsDir:       false,
		Path:        e.Path,
		Size:        e.Size,
	}
}
