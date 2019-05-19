package handler

import (
	"fmt"
	"net/http"

	"github.com/jinzhu/gorm"

	"github.com/sentriz/gonic/model"
	"github.com/sentriz/gonic/server/subsonic"
)

func (c *Controller) GetIndexes(w http.ResponseWriter, r *http.Request) {
	// we are browsing by folder, but the subsonic docs show sub <artist> elements
	// for this, so we're going to return root directories as "artists"
	var folders []*model.Folder
	c.DB.Where("parent_id = ?", 1).Find(&folders)
	var indexMap = make(map[rune]*subsonic.Index)
	var indexes []*subsonic.Index
	for _, folder := range folders {
		i := indexOf(folder.Name)
		index, ok := indexMap[i]
		if !ok {
			index = &subsonic.Index{
				Name:    string(i),
				Artists: []*subsonic.Artist{},
			}
			indexMap[i] = index
			indexes = append(indexes, index)
		}
		index.Artists = append(index.Artists, &subsonic.Artist{
			ID:   folder.ID,
			Name: folder.Name,
		})
	}
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
	childrenObj := []*subsonic.Child{}
	var cFolder model.Folder
	c.DB.First(&cFolder, id)
	//
	// start looking for child folders in the current dir
	var folders []*model.Folder
	c.DB.
		Where("parent_id = ?", id).
		Find(&folders)
	for _, folder := range folders {
		childrenObj = append(childrenObj, &subsonic.Child{
			Parent:  cFolder.ID,
			ID:      folder.ID,
			Title:   folder.Name,
			IsDir:   true,
			CoverID: folder.CoverID,
		})
	}
	//
	// start looking for child tracks in the current dir
	var tracks []*model.Track
	c.DB.
		Where("folder_id = ?", id).
		Preload("Album").
		Order("track_number").
		Find(&tracks)
	for _, track := range tracks {
		if getStrParam(r, "c") == "Jamstash" {
			// jamstash thinks it can't play flacs
			track.ContentType = "audio/mpeg"
			track.Suffix = "mp3"
		}
		childrenObj = append(childrenObj, &subsonic.Child{
			ID:          track.ID,
			Album:       track.Album.Title,
			Artist:      track.Artist,
			ContentType: track.ContentType,
			CoverID:     cFolder.CoverID,
			Duration:    0,
			IsDir:       false,
			Parent:      cFolder.ID,
			Path:        track.Path,
			Size:        track.Size,
			Suffix:      track.Suffix,
			Title:       track.Title,
			Track:       track.TrackNumber,
			Type:        "music",
		})
	}
	//
	// respond section
	sub := subsonic.NewResponse()
	sub.Directory = &subsonic.Directory{
		ID:       cFolder.ID,
		Parent:   cFolder.ParentID,
		Name:     cFolder.Name,
		Children: childrenObj,
	}
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
		// not sure what it meant by "artist" since we're browsing by folder
		// - so we'll consider the parent folder's name to be the "artist"
		q = q.Joins(`
			JOIN folders AS parent_folders
			ON folders.parent_id = parent_folders.id`)
		q = q.Order("parent_folders.name")
	case "alphabeticalByName":
		// not sure about "name" either, so lets use the folder's name
		q = q.Order("name")
	case "frequent":
		user := r.Context().Value(contextUserKey).(*model.User)
		q = q.Joins(`
			JOIN plays
			ON folders.id = plays.folder_id AND plays.user_id = ?`,
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
			ON folders.id = plays.folder_id AND plays.user_id = ?`,
			user.ID)
		q = q.Order("plays.time DESC")
	default:
		respondError(w, r, 10, fmt.Sprintf(
			"unknown value `%s` for parameter 'type'", listType,
		))
		return
	}
	var folders []*model.Folder
	q.
		Where("folders.has_tracks = 1").
		Offset(getIntParamOr(r, "offset", 0)).
		Limit(getIntParamOr(r, "size", 10)).
		Preload("Parent").
		Find(&folders)
	listObj := []*subsonic.Album{}
	for _, folder := range folders {
		listObj = append(listObj, &subsonic.Album{
			ID:       folder.ID,
			Title:    folder.Name,
			Album:    folder.Name,
			CoverID:  folder.CoverID,
			ParentID: folder.ParentID,
			IsDir:    true,
			Artist:   folder.Parent.Name,
		})
	}
	sub := subsonic.NewResponse()
	sub.Albums = &subsonic.Albums{
		List: listObj,
	}
	respond(w, r, sub)
}
