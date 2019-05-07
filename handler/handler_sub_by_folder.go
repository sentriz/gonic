package handler

import (
	"net/http"

	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/subsonic"
)

func (c *Controller) GetIndexes(w http.ResponseWriter, r *http.Request) {
	// we are browsing by folder, but the subsonic docs show sub <artist> elements
	// for this, so we're going to return root directories as "artists"
	var folders []*db.Folder
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
	var folders []*db.Folder
	c.DB.Where("parent_id = ?", id).Find(&folders)
	if len(folders) == 0 {
		respondError(w, r, 40, "couldn't find any directories")
		return
	}
	var cFolder db.Folder
	c.DB.First(&cFolder, id)
	sub := subsonic.NewResponse()
	sub.Directory = &subsonic.Directory{
		ID:     cFolder.ID,
		Parent: cFolder.ParentID,
		Name:   cFolder.Name,
	}
	for _, folder := range folders {
		sub.Directory.Children = append(sub.Directory.Children, &subsonic.Child{
			Parent: cFolder.ID,
			ID:     folder.ID,
			Title:  folder.Name,
			IsDir:  true,
		})
	}
	respond(w, r, sub)
}
