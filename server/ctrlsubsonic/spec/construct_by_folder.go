package spec

import (
	"path"

	"senan.xyz/g/gonic/model"
)

func NewAlbumByFolder(f *model.Album) *Album {
	a := &Album{
		Artist:     f.Parent.RightPath,
		ID:         f.ID,
		IsDir:      true,
		ParentID:   f.ParentID,
		Title:      f.RightPath,
		TrackCount: f.ChildCount,
	}
	if f.Cover != "" {
		a.CoverID = f.ID
	}
	return a
}

func NewTCAlbumByFolder(f *model.Album) *TrackChild {
	trCh := &TrackChild{
		ID:        f.ID,
		IsDir:     true,
		Title:     f.RightPath,
		ParentID:  f.ParentID,
		CreatedAt: f.UpdatedAt,
	}
	if f.Cover != "" {
		trCh.CoverID = f.ID
	}
	return trCh
}

func NewTCTrackByFolder(t *model.Track, parent *model.Album) *TrackChild {
	trCh := &TrackChild{
		ID:          t.ID,
		ContentType: t.MIME(),
		Suffix:      t.Ext(),
		Size:        t.Size,
		Artist:      t.TagTrackArtist,
		Title:       t.TagTitle,
		TrackNumber: t.TagTrackNumber,
		DiscNumber:  t.TagDiscNumber,
		Path: path.Join(
			parent.LeftPath,
			parent.RightPath,
			t.Filename,
		),
		ParentID:  parent.ID,
		Duration:  t.Length,
		Bitrate:   t.Bitrate,
		IsDir:     false,
		Type:      "music",
		CreatedAt: t.CreatedAt,
	}
	if parent.Cover != "" {
		trCh.CoverID = parent.ID
	}
	if t.Album != nil {
		trCh.Album = t.Album.RightPath
	}
	return trCh
}

func NewArtistByFolder(f *model.Album) *Artist {
	return &Artist{
		ID:         f.ID,
		Name:       f.RightPath,
		AlbumCount: f.ChildCount,
	}
}

func NewDirectoryByFolder(f *model.Album, children []*TrackChild) *Directory {
	return &Directory{
		ID:       f.ID,
		Parent:   f.ParentID,
		Name:     f.RightPath,
		Children: children,
	}
}
