package handler

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/jinzhu/gorm"

	"github.com/sentriz/gonic/model"
	"github.com/sentriz/gonic/server/subsonic"
)

// the subsonic spec metions "artist" a lot when talking about the
// browse by folder endpoints. but since we're not browsing by tag
// we can't access artists. so instead we'll consider the artist of
// an track to be the it's respective folder that comes directly
// under the root directory

func (c *Controller) GetIndexes(w http.ResponseWriter, r *http.Request) {
	var folders []*model.Album
	c.DB.
		Where("parent_id = 1").
		Find(&folders)
	indexMap := make(map[rune]*subsonic.Index)
	indexes := []*subsonic.Index{}
	for _, folder := range folders {
		i := indexOf(folder.RightPath)
		index, ok := indexMap[i]
		if !ok {
			index = &subsonic.Index{
				Name:    string(i),
				Artists: []*subsonic.Artist{},
			}
			indexMap[i] = index
			indexes = append(indexes, index)
		}
		index.Artists = append(index.Artists,
			makeArtistFromFolder(folder))
	}
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i].Name < indexes[j].Name
	})
	sub := subsonic.NewResponse()
	sub.Indexes = &subsonic.Indexes{
		LastModified: 0,
		Index:        indexes,
	}
	respond(w, r, sub)
}

func (c *Controller) GetMusicDirectory(w http.ResponseWriter, r *http.Request) {
	id, err := getIntParam(r, "id")
	if err != nil {
		respondError(w, r, 10, "please provide an `id` parameter")
		return
	}
	childrenObj := []*subsonic.Track{}
	folder := &model.Album{}
	c.DB.First(folder, id)
	//
	// start looking for child childFolders in the current dir
	var childFolders []*model.Album
	c.DB.
		Where("parent_id = ?", id).
		Find(&childFolders)
	for _, c := range childFolders {
		childrenObj = append(childrenObj,
			makeChildFromFolder(c, folder))
	}
	//
	// start looking for child childTracks in the current dir
	var childTracks []*model.Track
	c.DB.
		Where("album_id = ?", id).
		Preload("Album").
		Order("filename").
		Find(&childTracks)
	for _, c := range childTracks {
		toAppend := makeChildFromTrack(c, folder)
		if getStrParam(r, "c") == "Jamstash" {
			// jamstash thinks it can't play flacs
			toAppend.ContentType = "audio/mpeg"
			toAppend.Suffix = "mp3"
		}
		childrenObj = append(childrenObj, toAppend)
	}
	//
	// respond section
	sub := subsonic.NewResponse()
	sub.Directory = makeDirFromFolder(folder, childrenObj)
	respond(w, r, sub)
}

// changes to this function should be reflected in in _by_tags.go's
// getAlbumListTwo() function
func (c *Controller) GetAlbumList(w http.ResponseWriter, r *http.Request) {
	listType := getStrParam(r, "type")
	if listType == "" {
		respondError(w, r, 10, "please provide a `type` parameter")
		return
	}
	q := c.DB
	switch listType {
	case "alphabeticalByArtist":
		q = q.Joins(`
			JOIN albums AS parent_albums
			ON albums.parent_id = parent_albums.id`)
		q = q.Order("parent_albums.right_path")
	case "alphabeticalByName":
		q = q.Order("right_path")
	case "frequent":
		user := r.Context().Value(contextUserKey).(*model.User)
		q = q.Joins(`
			JOIN plays
			ON albums.id = plays.album_id AND plays.user_id = ?`,
			user.ID)
		q = q.Order("plays.count DESC")
	case "newest":
		q = q.Order("updated_at DESC")
	case "random":
		q = q.Order(gorm.Expr("random()"))
	case "recent":
		user := r.Context().Value(contextUserKey).(*model.User)
		q = q.Joins(`
			JOIN plays
			ON albums.id = plays.album_id AND plays.user_id = ?`,
			user.ID)
		q = q.Order("plays.time DESC")
	default:
		respondError(w, r, 10,
			"unknown value `%s` for parameter 'type'", listType)
		return
	}
	var folders []*model.Album
	q.
		Where("albums.tag_artist_id IS NOT NULL").
		Offset(getIntParamOr(r, "offset", 0)).
		Limit(getIntParamOr(r, "size", 10)).
		Preload("Parent").
		Find(&folders)
	sub := subsonic.NewResponse()
	sub.Albums = &subsonic.Albums{}
	for _, folder := range folders {
		sub.Albums.List = append(sub.Albums.List,
			makeAlbumFromFolder(folder))
	}
	respond(w, r, sub)
}

func (c *Controller) SearchTwo(w http.ResponseWriter, r *http.Request) {
	query := getStrParam(r, "query")
	if query == "" {
		respondError(w, r, 10, "please provide a `query` parameter")
		return
	}
	query = fmt.Sprintf("%%%s%%",
		strings.TrimSuffix(query, "*"))
	results := &subsonic.SearchResultTwo{}
	//
	// search "artists"
	var artists []*model.Album
	c.DB.
		Where("parent_id = 1 AND right_path LIKE ?", query).
		Offset(getIntParamOr(r, "artistOffset", 0)).
		Limit(getIntParamOr(r, "artistCount", 20)).
		Find(&artists)
	for _, a := range artists {
		results.Artists = append(results.Artists,
			makeDirFromFolder(a, nil))
	}
	//
	// search "albums"
	var albums []*model.Album
	c.DB.
		Preload("Parent").
		Where("tag_artist_id IS NOT NULL AND right_path LIKE ?", query).
		Offset(getIntParamOr(r, "albumOffset", 0)).
		Limit(getIntParamOr(r, "albumCount", 20)).
		Find(&albums)
	for _, a := range albums {
		results.Albums = append(results.Albums,
			makeChildFromFolder(a, a.Parent))
	}
	//
	// search tracks
	var tracks []*model.Track
	c.DB.
		Preload("Album").
		Where("filename LIKE ?", query).
		Offset(getIntParamOr(r, "songOffset", 0)).
		Limit(getIntParamOr(r, "songCount", 20)).
		Find(&tracks)
	for _, t := range tracks {
		results.Tracks = append(results.Tracks,
			makeChildFromTrack(t, t.Album))
	}
	//
	sub := subsonic.NewResponse()
	sub.SearchResultTwo = results
	respond(w, r, sub)
}
