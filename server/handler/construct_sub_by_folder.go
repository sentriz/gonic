package handler

import (
	"path"

	"github.com/sentriz/gonic/model"
	"github.com/sentriz/gonic/server/subsonic"
)

func newAlbumByFolder(f *model.Album) *subsonic.Album {
	return &subsonic.Album{
		Artist:   f.Parent.RightPath,
		CoverID:  f.ID,
		ID:       f.ID,
		IsDir:    true,
		ParentID: f.ParentID,
		Title:    f.RightPath,
	}
}

func newTCAlbumByFolder(f *model.Album, parent *model.Album) *subsonic.TrackChild {
	child := &subsonic.TrackChild{
		CoverID: f.ID,
		ID:      f.ID,
		IsDir:   true,
		Title:   f.RightPath,
	}
	if parent != nil {
		child.ParentID = parent.ID
	}
	return child
}

func newTCTrackByFolder(t *model.Track, parent *model.Album) *subsonic.TrackChild {
	return &subsonic.TrackChild{
		ID:          t.ID,
		Album:       t.Album.RightPath,
		ContentType: t.MIME(),
		Suffix:      t.Ext(),
		Size:        t.Size,
		Artist:      t.TagTrackArtist,
		Title:       t.TagTitle,
		TrackNumber: t.TagTrackNumber,
		Path: path.Join(
			parent.LeftPath,
			parent.RightPath,
			t.Filename,
		),
		ParentID: parent.ID,
		CoverID:  parent.ID,
		Duration: t.Duration,
		Bitrate:  t.Bitrate,
		IsDir:    false,
		Type:     "music",
	}
}

func newArtistByFolder(f *model.Album) *subsonic.Artist {
	return &subsonic.Artist{
		ID:   f.ID,
		Name: f.RightPath,
	}
}

func newDirectoryByFolder(f *model.Album, children []*subsonic.TrackChild) *subsonic.Directory {
	return &subsonic.Directory{
		ID:       f.ID,
		Parent:   f.ParentID,
		Name:     f.RightPath,
		Children: children,
	}
}
