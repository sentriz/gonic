package handler

import (
	"github.com/sentriz/gonic/model"
	"github.com/sentriz/gonic/server/subsonic"
)

func makeChildFromFolder(f *model.Folder, parent *model.Folder) *subsonic.Child {
	return &subsonic.Child{
		ID:       f.ID,
		Title:    f.Name,
		CoverID:  f.CoverID,
		ParentID: parent.ID,
		IsDir:    true,
	}
}

func makeChildFromTrack(t *model.Track, parent *model.Folder) *subsonic.Child {
	return &subsonic.Child{
		ID:          t.ID,
		Album:       t.Album.Title,
		Artist:      t.TrackArtist,
		ContentType: t.ContentType,
		Path:        t.Path,
		Size:        t.Size,
		Suffix:      t.Suffix,
		Title:       t.Title,
		Track:       t.TrackNumber,
		ParentID:    parent.ID,
		CoverID:     parent.CoverID,
		Duration:    0,
		IsDir:       false,
		Type:        "music",
	}
}

func makeAlbumFromFolder(f *model.Folder) *subsonic.Album {
	return &subsonic.Album{
		ID:       f.ID,
		Title:    f.Name,
		Album:    f.Name,
		CoverID:  f.CoverID,
		ParentID: f.ParentID,
		Artist:   f.Parent.Name,
		IsDir:    true,
	}
}

func makeArtistFromFolder(f *model.Folder) *subsonic.Artist {
	return &subsonic.Artist{
		ID:   f.ID,
		Name: f.Name,
	}
}
