package handler

import (
	"fmt"
	"net/http"

	"github.com/jinzhu/gorm"

	"github.com/sentriz/gonic/model"
	"github.com/sentriz/gonic/server/subsonic"
)

func (c *Controller) GetArtists(w http.ResponseWriter, r *http.Request) {
	var artists []*model.AlbumArtist
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
		index.Artists = append(index.Artists, &subsonic.Artist{
			ID:   *artist.ID,
			Name: artist.Name,
		})
	}
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
	var artist model.AlbumArtist
	c.DB.
		Preload("Albums").
		First(&artist, id)
	albumsObj := []*subsonic.Album{}
	for _, album := range artist.Albums {
		albumsObj = append(albumsObj, &subsonic.Album{
			ID:       *album.ID,
			Name:     album.Title,
			Created:  album.CreatedAt,
			Artist:   artist.Name,
			ArtistID: *artist.ID,
			CoverID:  *album.CoverID,
		})
	}
	sub := subsonic.NewResponse()
	sub.Artist = &subsonic.Artist{
		ID:     *artist.ID,
		Name:   artist.Name,
		Albums: albumsObj,
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
	c.DB.
		Preload("AlbumArtist").
		Preload("Tracks").
		First(&album, id)
	tracksObj := []*subsonic.Track{}
	for _, track := range album.Tracks {
		tracksObj = append(tracksObj, &subsonic.Track{
			ID:          *track.ID,
			Title:       track.Title,
			Artist:      track.Artist, // track artist
			TrackNo:     track.TrackNumber,
			ContentType: track.ContentType,
			Path:        track.Path,
			Suffix:      track.Suffix,
			Created:     track.CreatedAt,
			Size:        track.Size,
			Album:       album.Title,
			AlbumID:     *album.ID,
			ArtistID:    *album.AlbumArtist.ID, // album artist
			CoverID:     *album.CoverID,
			Type:        "music",
		})
	}
	sub := subsonic.NewResponse()
	sub.Album = &subsonic.Album{
		ID:      *album.ID,
		Name:    album.Title,
		CoverID: *album.CoverID,
		Created: album.CreatedAt,
		Artist:  album.AlbumArtist.Name,
		Tracks:  tracksObj,
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
			JOIN album_artists
			ON albums.album_artist_id = album_artists.id`)
		q = q.Order("album_artists.name")
	case "alphabeticalByName":
		q = q.Order("title")
	case "byYear":
		q = q.Where(
			"year BETWEEN ? AND ?",
			getIntParamOr(r, "fromYear", 1800),
			getIntParamOr(r, "toYear", 2200))
		q = q.Order("year")
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
		respondError(w, r, 10, fmt.Sprintf(
			"unknown value `%s` for parameter 'type'", listType,
		))
		return
	}
	var albums []*model.Album
	q.
		Offset(getIntParamOr(r, "offset", 0)).
		Limit(getIntParamOr(r, "size", 10)).
		Preload("AlbumArtist").
		Find(&albums)
	listObj := []*subsonic.Album{}
	for _, album := range albums {
		listObj = append(listObj, &subsonic.Album{
			ID:       *album.ID,
			Name:     album.Title,
			Created:  album.CreatedAt,
			CoverID:  *album.CoverID,
			Artist:   album.AlbumArtist.Name,
			ArtistID: *album.AlbumArtist.ID,
		})
	}
	sub := subsonic.NewResponse()
	sub.AlbumsTwo = &subsonic.Albums{
		List: listObj,
	}
	respond(w, r, sub)
}
