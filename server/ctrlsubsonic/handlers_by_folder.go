package ctrlsubsonic

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/db"
)

// the subsonic spec metions "artist" a lot when talking about the
// browse by folder endpoints. but since we're not browsing by tag
// we can't access artists. so instead we'll consider the artist of
// an track to be the it's respective folder that comes directly
// under the root directory

func (c *Controller) ServeGetIndexes(r *http.Request) *spec.Response {
	var folders []*db.Album
	c.DB.
		Select("*, count(sub.id) child_count").
		Joins("LEFT JOIN albums sub ON albums.id=sub.parent_id").
		Where("albums.parent_id=1").
		Group("albums.id").
		Order("albums.right_path COLLATE NOCASE").
		Find(&folders)
	// [a-z#] -> 27
	indexMap := make(map[string]*spec.Index, 27)
	resp := make([]*spec.Index, 0, 27)
	for _, folder := range folders {
		i := lowerUDecOrHash(folder.IndexRightPath())
		index, ok := indexMap[i]
		if !ok {
			index = &spec.Index{
				Name:    i,
				Artists: []*spec.Artist{},
			}
			indexMap[i] = index
			resp = append(resp, index)
		}
		index.Artists = append(index.Artists,
			spec.NewArtistByFolder(folder))
	}
	sub := spec.NewResponse()
	sub.Indexes = &spec.Indexes{
		LastModified: 0,
		Index:        resp,
	}
	return sub
}

func (c *Controller) ServeGetMusicDirectory(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetInt("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	childrenObj := []*spec.TrackChild{}
	folder := &db.Album{}
	c.DB.First(folder, id)
	// ** begin start looking for child childFolders in the current dir
	var childFolders []*db.Album
	c.DB.
		Where("parent_id=?", id).
		Order("albums.right_path COLLATE NOCASE").
		Find(&childFolders)
	for _, c := range childFolders {
		childrenObj = append(childrenObj, spec.NewTCAlbumByFolder(c))
	}
	// ** begin start looking for child childTracks in the current dir
	var childTracks []*db.Track
	c.DB.
		Where("album_id=?", id).
		Preload("Album").
		Order("filename").
		Find(&childTracks)
	for _, c := range childTracks {
		toAppend := spec.NewTCTrackByFolder(c, folder)
		if v, _ := params.Get("c"); v == "Jamstash" {
			// jamstash thinks it can't play flacs
			toAppend.ContentType = "audio/mpeg"
			toAppend.Suffix = "mp3"
		}
		childrenObj = append(childrenObj, toAppend)
	}
	// ** begin respond section
	sub := spec.NewResponse()
	sub.Directory = spec.NewDirectoryByFolder(folder, childrenObj)
	return sub
}

// ServeGetAlbumList handles the getAlbumList view.
// changes to this function should be reflected in in _by_tags.go's
// getAlbumListTwo() function
func (c *Controller) ServeGetAlbumList(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	q := c.DB.DB
	switch v, _ := params.Get("type"); v {
	case "alphabeticalByArtist":
		q = q.Joins(`
			JOIN albums parent_albums
			ON albums.parent_id=parent_albums.id`)
		q = q.Order("parent_albums.right_path")
	case "alphabeticalByName":
		q = q.Order("right_path")
	case "frequent":
		user := r.Context().Value(CtxUser).(*db.User)
		q = q.Joins(`
			JOIN plays
			ON albums.id=plays.album_id AND plays.user_id=?`,
			user.ID)
		q = q.Order("plays.count DESC")
	case "newest":
		q = q.Order("modified_at DESC")
	case "random":
		q = q.Order(gorm.Expr("random()"))
	case "recent":
		user := r.Context().Value(CtxUser).(*db.User)
		q = q.Joins(`
			JOIN plays
			ON albums.id=plays.album_id AND plays.user_id=?`,
			user.ID)
		q = q.Order("plays.time DESC")
	default:
		return spec.NewError(10, "unknown value `%s` for parameter 'type'", v)
	}
	var folders []*db.Album
	// TODO: think about removing this extra join to count number
	// of children. it might make sense to store that in the db
	q.
		Select("albums.*, count(tracks.id) child_count").
		Joins("LEFT JOIN tracks ON tracks.album_id=albums.id").
		Group("albums.id").
		Where("albums.tag_artist_id IS NOT NULL").
		Offset(params.GetOrInt("offset", 0)).
		Limit(params.GetOrInt("size", 10)).
		Preload("Parent").
		Find(&folders)
	sub := spec.NewResponse()
	sub.Albums = &spec.Albums{
		List: make([]*spec.Album, len(folders)),
	}
	for i, folder := range folders {
		sub.Albums.List[i] = spec.NewAlbumByFolder(folder)
	}
	return sub
}

func (c *Controller) ServeSearchTwo(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	query, err := params.Get("query")
	if err != nil {
		return spec.NewError(10, "please provide a `query` parameter")
	}
	query = fmt.Sprintf("%%%s%%", strings.TrimSuffix(query, "*"))
	results := &spec.SearchResultTwo{}
	// ** begin search "artists"
	var artists []*db.Album
	c.DB.
		Where(`
			parent_id=1
			AND (	right_path LIKE ? OR
					right_path_u_dec LIKE ?	)`,
			query, query).
		Offset(params.GetOrInt("artistOffset", 0)).
		Limit(params.GetOrInt("artistCount", 20)).
		Find(&artists)
	for _, a := range artists {
		results.Artists = append(results.Artists,
			spec.NewDirectoryByFolder(a, nil))
	}
	// ** begin search "albums"
	var albums []*db.Album
	c.DB.
		Where(`
			tag_artist_id IS NOT NULL
			AND (	right_path LIKE ? OR
					right_path_u_dec LIKE ?	)`,
			query, query).
		Offset(params.GetOrInt("albumOffset", 0)).
		Limit(params.GetOrInt("albumCount", 20)).
		Find(&albums)
	for _, a := range albums {
		results.Albums = append(results.Albums, spec.NewTCAlbumByFolder(a))
	}
	// ** begin search tracks
	var tracks []*db.Track
	c.DB.
		Preload("Album").
		Where("filename LIKE ? OR filename_u_dec LIKE ?",
			query, query).
		Offset(params.GetOrInt("songOffset", 0)).
		Limit(params.GetOrInt("songCount", 20)).
		Find(&tracks)
	for _, t := range tracks {
		results.Tracks = append(results.Tracks,
			spec.NewTCTrackByFolder(t, t.Album))
	}
	//
	sub := spec.NewResponse()
	sub.SearchResultTwo = results
	return sub
}
