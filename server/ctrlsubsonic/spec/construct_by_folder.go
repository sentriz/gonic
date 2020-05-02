package spec

import (
	"path"

	"go.senan.xyz/gonic/server/db"
)

func NewAlbumByFolder(f *db.Album) *Album {
	a := &Album{
		Artist:     f.Parent.RightPath,
		ID:         f.SID(),
		IsDir:      true,
		ParentID:   f.ParentSID(),
		Title:      f.RightPath,
		TrackCount: f.ChildCount,
	}
	if f.Cover != "" {
		a.CoverID = f.SID()
	}
	return a
}

func NewTCAlbumByFolder(f *db.Album) *TrackChild {
	trCh := &TrackChild{
		ID:        f.SID(),
		IsDir:     true,
		Title:     f.RightPath,
		ParentID:  f.ParentSID(),
		CreatedAt: f.UpdatedAt,
	}
	if f.Cover != "" {
		trCh.CoverID = f.SID()
	}
	return trCh
}

func NewTCTrackByFolder(t *db.Track, parent *db.Album) *TrackChild {
	trCh := &TrackChild{
		ID:          t.SID(),
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
		ParentID:  parent.SID(),
		Duration:  t.Length,
		Bitrate:   t.Bitrate,
		IsDir:     false,
		Type:      "music",
		CreatedAt: t.CreatedAt,
	}
	if parent.Cover != "" {
		trCh.CoverID = parent.SID()
	}
	if t.Album != nil {
		trCh.Album = t.Album.RightPath
	}
	return trCh
}

func NewArtistByFolder(f *db.Album) *Artist {
	// the db is structued around "browse by tags", and where
	// an album is also a folder. so we're constructing an artist
	// from an "album" where
	// maybe TODO: rename the Album model to Folder
	return &Artist{
		ID:         f.SID(),
		Name:       f.RightPath,
		AlbumCount: f.ChildCount,
	}
}

func NewDirectoryByFolder(f *db.Album, children []*TrackChild) *Directory {
	dir := &Directory{
		ID:       f.SID(),
		Name:     f.RightPath,
		Children: children,
	}
	// don't show the root dir as a parent
	if f.ParentID != 1 {
		dir.ParentID = f.ParentSID()
	}
	return dir
}
