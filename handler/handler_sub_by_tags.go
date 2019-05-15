package handler

import (
	"fmt"
	"net/http"

	"github.com/jinzhu/gorm"

	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/subsonic"
)

func (c *Controller) GetArtists(w http.ResponseWriter, r *http.Request) {
	var artists []*db.AlbumArtist
	c.DB.Find(&artists)
	var indexMap = make(map[rune]*subsonic.Index)
	var indexes []*subsonic.Index
	for _, artist := range artists {
		i := indexOf(artist.Name)
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
			ID:   artist.ID,
			Name: artist.Name,
		})
	}
	sub := subsonic.NewResponse()
	sub.Artists = indexes
	respond(w, r, sub)
}

func (c *Controller) GetArtist(w http.ResponseWriter, r *http.Request) {
	id, err := getIntParam(r, "id")
	if err != nil {
		respondError(w, r, 10, "please provide an `id` parameter")
		return
	}
	var artist db.AlbumArtist
	c.DB.
		Preload("Albums").
		First(&artist, id)
	sub := subsonic.NewResponse()
	sub.Artist = &subsonic.Artist{
		ID:   artist.ID,
		Name: artist.Name,
	}
	for _, album := range artist.Albums {
		sub.Artist.Albums = append(sub.Artist.Albums, &subsonic.Album{
			ID:       album.ID,
			Name:     album.Title,
			Created:  album.CreatedAt,
			Artist:   artist.Name,
			ArtistID: artist.ID,
			CoverID:  album.CoverID,
		})
	}
	respond(w, r, sub)
}

func (c *Controller) GetAlbum(w http.ResponseWriter, r *http.Request) {
	id, err := getIntParam(r, "id")
	if err != nil {
		respondError(w, r, 10, "please provide an `id` parameter")
		return
	}
	var album db.Album
	c.DB.
		Preload("AlbumArtist").
		Preload("Tracks").
		First(&album, id)
	sub := subsonic.NewResponse()
	sub.Album = &subsonic.Album{
		ID:      album.ID,
		Name:    album.Title,
		CoverID: album.CoverID,
		Created: album.CreatedAt,
		Artist:  album.AlbumArtist.Name,
	}
	for _, track := range album.Tracks {
		sub.Album.Tracks = append(sub.Album.Tracks, &subsonic.Track{
			ID:          track.ID,
			Title:       track.Title,
			Artist:      track.Artist, // track artist
			TrackNo:     track.TrackNumber,
			ContentType: track.ContentType,
			Path:        track.Path,
			Suffix:      track.Suffix,
			Created:     track.CreatedAt,
			Size:        track.Size,
			Album:       album.Title,
			AlbumID:     album.ID,
			ArtistID:    album.AlbumArtist.ID, // album artist
			CoverID:     album.CoverID,
			Type:        "music",
		})
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
	query := c.DB
	switch listType {
	case "alphabeticalByArtist":
		query = query.
			Joins("JOIN album_artists ON albums.album_artist_id=album_artists.id").
			Order("album_artists.name")
	case "alphabeticalByName":
		query = query.Order("title")
	case "byYear":
		startYear := getIntParamOr(r, "fromYear", 1800)
		endYear := getIntParamOr(r, "toYear", 2200)
		query = query.
			Where("year BETWEEN ? AND ?", startYear, endYear).
			Order("year")
	case "frequent":
		user := r.Context().Value(contextUserKey).(*db.User)
		query = query.
			Joins("JOIN plays ON albums.id=plays.album_id AND plays.user_id=?", user.ID).
			Order("plays.count desc")
	case "newest":
		query = query.Order("updated_at desc")
	case "random":
		query = query.Order(gorm.Expr("random()"))
	case "recent":
		user := r.Context().Value(contextUserKey).(*db.User)
		query = query.
			Joins("JOIN plays ON albums.id=plays.album_id AND plays.user_id=?", user.ID).
			Order("plays.time desc")
	default:
		respondError(w, r, 10, fmt.Sprintf(
			"unknown value `%s` for parameter 'type'", listType,
		))
		return
	}
	offset := getIntParamOr(r, "offset", 0)
	size := getIntParamOr(r, "size", 10)
	var albums []*db.Album
	query.
		Offset(offset).
		Limit(size).
		Preload("AlbumArtist").
		Find(&albums)
	sub := subsonic.NewResponse()
	for _, album := range albums {
		sub.Albums = append(sub.Albums, &subsonic.Album{
			ID:       album.ID,
			Name:     album.Title,
			Created:  album.CreatedAt,
			CoverID:  album.CoverID,
			Artist:   album.AlbumArtist.Name,
			ArtistID: album.AlbumArtist.ID,
		})
	}
	respond(w, r, sub)
}
