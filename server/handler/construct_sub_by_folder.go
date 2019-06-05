package handler

import (
	"path"

	"github.com/sentriz/gonic/model"
	"github.com/sentriz/gonic/server/subsonic"
)

func makeChildFromFolder(f *model.Album, parent *model.Album) *subsonic.Track {
	child := &subsonic.Track{
		ID:      f.ID,
		CoverID: f.ID,
		Title:   f.RightPath,
		IsDir:   true,
	}
	if parent != nil {
		child.ParentID = parent.ID
	}
	return child
}

func makeChildFromTrack(t *model.Track, parent *model.Album) *subsonic.Track {
	return &subsonic.Track{
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
		Duration: 0,
		IsDir:    false,
		Type:     "music",
	}
}

func makeAlbumFromFolder(f *model.Album) *subsonic.Album {
	return &subsonic.Album{
		ID:       f.ID,
		Title:    f.RightPath,
		CoverID:  f.ID,
		ParentID: f.ParentID,
		Artist:   f.Parent.RightPath,
		IsDir:    true,
	}
}

func makeArtistFromFolder(f *model.Album) *subsonic.Artist {
	return &subsonic.Artist{
		ID:   f.ID,
		Name: f.RightPath,
	}
}

func makeDirFromFolder(f *model.Album, children []*subsonic.Track) *subsonic.Directory {
	return &subsonic.Directory{
		ID:       f.ID,
		Parent:   f.ParentID,
		Name:     f.RightPath,
		Children: children,
	}
}
