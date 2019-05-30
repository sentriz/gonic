package handler

import (
	"github.com/sentriz/gonic/model"
	"github.com/sentriz/gonic/server/subsonic"
)

func makeChildFromFolder(f *model.Folder, parent *model.Folder) *subsonic.Track {
	child := &subsonic.Track{
		ID:      f.ID,
		Title:   f.Name,
		CoverID: f.CoverID,
		IsDir:   true,
	}
	if parent != nil {
		child.ParentID = parent.ID
	}
	return child
}

func makeChildFromTrack(t *model.Track, parent *model.Folder) *subsonic.Track {
	return &subsonic.Track{
		ID:          t.ID,
		Album:       t.Album.Title,
		Artist:      t.TrackArtist,
		ContentType: t.ContentType,
		Path:        t.Path,
		Size:        t.Size,
		Suffix:      t.Suffix,
		Title:       t.Title,
		TrackNumber: t.TrackNumber,
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

func makeDirFromFolder(f *model.Folder, children []*subsonic.Track) *subsonic.Directory {
	return &subsonic.Directory{
		ID:       f.ID,
		Parent:   f.ParentID,
		Name:     f.Name,
		Children: children,
	}
}
