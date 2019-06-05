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

func (c *Controller) GetArtists(w http.ResponseWriter, r *http.Request) {
	var artists []model.Artist
	c.DB.Find(&artists)
	var indexMap = make(map[rune]*subsonic.Index)
	var indexes subsonic.Artists
	for _, artist := range artists {
		i := indexOf(artist.Name)
		index, ok := indexMap[i]
		if !ok {
			index = &subsonic.Index{
				Name:    string(i),
				Artists: []*subsonic.Artist{},
			}
			indexMap[i] = index
			indexes.List = append(indexes.List, index)
		}
		index.Artists = append(index.Artists,
			makeArtistFromArtist(&artist))
	}
	sort.Slice(indexes.List, func(i, j int) bool {
		return indexes.List[i].Name < indexes.List[j].Name
	})
	sub := subsonic.NewResponse()
	sub.Artists = &indexes
	respond(w, r, sub)
}

func (c *Controller) GetArtist(w http.ResponseWriter, r *http.Request) {
	id, err := getIntParam(r, "id")
	if err != nil {
		respondError(w, r, 10, "please provide an `id` parameter")
		return
	}
	var artist model.Artist
	c.DB.
		Preload("Albums").
		First(&artist, id)
	sub := subsonic.NewResponse()
	sub.Artist = makeArtistFromArtist(&artist)
	for _, album := range artist.Albums {
		sub.Artist.Albums = append(sub.Artist.Albums,
			makeAlbumFromAlbum(&album, &artist))
	}
	respond(w, r, sub)
}

func (c *Controller) GetAlbum(w http.ResponseWriter, r *http.Request) {
	id, err := getIntParam(r, "id")
	if err != nil {
		respondError(w, r, 10, "please provide an `id` parameter")
		return
	}
	var album model.Album
	err = c.DB.
		Preload("TagArtist").
		Preload("Tracks", func(db *gorm.DB) *gorm.DB {
			return db.Order("tracks.tag_track_number")
		}).
		First(&album, id).
		Error
	if gorm.IsRecordNotFoundError(err) {
		respondError(w, r, 10, "couldn't find an album with that id")
		return
	}
	sub := subsonic.NewResponse()
	sub.Album = makeAlbumFromAlbum(&album, &album.TagArtist)
	for _, track := range album.Tracks {
		sub.Album.Tracks = append(sub.Album.Tracks,
			makeTrackFromTrack(&track, &album))
	}
	respond(w, r, sub)
}

// changes to this function should be reflected in in _by_folder.go's
// getAlbumList() function
func (c *Controller) GetAlbumListTwo(w http.ResponseWriter, r *http.Request) {
	listType := getStrParam(r, "type")
	if listType == "" {
		respondError(w, r, 10, "please provide a `type` parameter")
		return
	}
	q := c.DB
	switch listType {
	case "alphabeticalByArtist":
		q = q.Joins(`
			JOIN artists
			ON albums.tag_artist_id = artists.id`)
		q = q.Order("artists.name")
	case "alphabeticalByName":
		q = q.Order("tag_title")
	case "byYear":
		q = q.Where(
			"tag_year BETWEEN ? AND ?",
			getIntParamOr(r, "fromYear", 1800),
			getIntParamOr(r, "toYear", 2200))
		q = q.Order("tag_year")
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
	var albums []model.Album
	q.
		Where("albums.tag_artist_id IS NOT NULL").
		Offset(getIntParamOr(r, "offset", 0)).
		Limit(getIntParamOr(r, "size", 10)).
		Preload("TagArtist").
		Find(&albums)
	sub := subsonic.NewResponse()
	sub.AlbumsTwo = &subsonic.Albums{}
	for _, album := range albums {
		sub.AlbumsTwo.List = append(sub.AlbumsTwo.List,
			makeAlbumFromAlbum(&album, &album.TagArtist))
	}
	respond(w, r, sub)
}

func (c *Controller) SearchThree(w http.ResponseWriter, r *http.Request) {
	query := getStrParam(r, "query")
	if query == "" {
		respondError(w, r, 10, "please provide a `query` parameter")
		return
	}
	query = fmt.Sprintf("%%%s%%",
		strings.TrimSuffix(query, "*"))
	results := &subsonic.SearchResultThree{}
	//
	// search "artists"
	var artists []model.Artist
	c.DB.
		Where("name LIKE ?", query).
		Offset(getIntParamOr(r, "artistOffset", 0)).
		Limit(getIntParamOr(r, "artistCount", 20)).
		Find(&artists)
	for _, a := range artists {
		results.Artists = append(results.Artists,
			makeArtistFromArtist(&a))
	}
	//
	// search "albums"
	var albums []model.Album
	c.DB.
		Preload("TagArtist").
		Where("tag_title LIKE ?", query).
		Offset(getIntParamOr(r, "albumOffset", 0)).
		Limit(getIntParamOr(r, "albumCount", 20)).
		Find(&albums)
	for _, a := range albums {
		results.Albums = append(results.Albums,
			makeAlbumFromAlbum(&a, &a.TagArtist))
	}
	//
	// search tracks
	var tracks []model.Track
	c.DB.
		Preload("Album").
		Where("tag_title LIKE ?", query).
		Offset(getIntParamOr(r, "songOffset", 0)).
		Limit(getIntParamOr(r, "songCount", 20)).
		Find(&tracks)
	for _, t := range tracks {
		results.Tracks = append(results.Tracks,
			makeTrackFromTrack(&t, &t.Album))
	}
	sub := subsonic.NewResponse()
	sub.SearchResultThree = results
	respond(w, r, sub)
}
