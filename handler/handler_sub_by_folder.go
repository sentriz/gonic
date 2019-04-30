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
	sub.Artists = indexes
	respond(w, r, sub)
}

func (c *Controller) GetMusicDirectory(w http.ResponseWriter, r *http.Request) {
}
