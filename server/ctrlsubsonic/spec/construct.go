package spec

import "senan.xyz/g/gonic/model"

func NewPlaylist(p *model.Playlist) *Playlist {
	return &Playlist{
		ID:      p.ID,
		Name:    p.Name,
		Comment: p.Comment,
	}
}
