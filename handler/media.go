package handler

import (
	"fmt"
	"net/http"
	"os"
	"unicode"

	"github.com/jinzhu/gorm"
	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/subsonic"

	"github.com/mozillazg/go-unidecode"
)

var orderExpr = map[string]interface{}{
	"random":               gorm.Expr("random()"),
	"newest":               "updated_at desc",
	"alphabeticalByName":   "title",
	"alphabeticalByArtist": "album_artist.name",
}

func indexOf(s string) rune {
	first := string(s[0])
	c := rune(unidecode.Unidecode(first)[0])
	if !unicode.IsLetter(c) {
		return '#'
	}
	return c
}

func (c *Controller) Ping(w http.ResponseWriter, r *http.Request) {
	sub := subsonic.NewResponse()
	respond(w, r, sub)
}

func (c *Controller) GetCoverArt(w http.ResponseWriter, r *http.Request) {
	id, err := getIntParam(r, "id")
	if err != nil {
		respondError(w, r, 10, "please provide an `id` parameter")
		return
	}
	var cover db.Cover
	c.DB.First(&cover, id)
	w.Write(cover.Image)
}

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
			CoverID:  album.ID,
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
		CoverID: album.ID,
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
			CoverID:     album.ID,
			Type:        "music",
		})
	}
	respond(w, r, sub)
}

func (c *Controller) GetMusicFolders(w http.ResponseWriter, r *http.Request) {
	sub := subsonic.NewResponse()
	sub.MusicFolders = []*subsonic.MusicFolder{
		{ID: 0, Name: "music"},
	}
	respond(w, r, sub)
}

func (c *Controller) GetAlbumList(w http.ResponseWriter, r *http.Request) {
	listType := getStrParam(r, "type")
	if listType == "" {
		respondError(w, r, 10, "please provide a `type` parameter")
		return
	}
	orderType, ok := orderExpr[listType]
	if !ok {
		respondError(w, r, 10, fmt.Sprintf(
			"unknown value `%s` for parameter 'type'", listType,
		))
		return
	}
	size := getIntParamOr(r, "size", 10)
	var albums []*db.Album
	c.DB.
		Preload("AlbumArtist").
		Order(orderType).
		Limit(size).
		Find(&albums)
	sub := subsonic.NewResponse()
	for _, album := range albums {
		sub.Albums = append(sub.Albums, &subsonic.Album{
			ID:       album.ID,
			Name:     album.Title,
			Created:  album.CreatedAt,
			CoverID:  album.ID,
			Artist:   album.AlbumArtist.Name,
			ArtistID: album.AlbumArtist.ID,
		})
	}
	respond(w, r, sub)
}

func (c *Controller) Stream(w http.ResponseWriter, r *http.Request) {
	id, err := getIntParam(r, "id")
	if err != nil {
		respondError(w, r, 10, "please provide an `id` parameter")
		return
	}
	var track db.Track
	c.DB.First(&track, id)
	if track.Path == "" {
		respondError(w, r, 70, fmt.Sprintf("media with id `%d` was not found", id))
		return
	}
	file, err := os.Open(track.Path)
	if err != nil {
		respondError(w, r, 0, fmt.Sprintf("error while streaming media: %v", err))
		return
	}
	stat, _ := file.Stat()
	http.ServeContent(w, r, track.Path, stat.ModTime(), file)
}

func (c *Controller) GetLicence(w http.ResponseWriter, r *http.Request) {
	sub := subsonic.NewResponse()
	sub.Licence = &subsonic.Licence{
		Valid: true,
	}
	respond(w, r, sub)
}

func (c *Controller) NotFound(w http.ResponseWriter, r *http.Request) {
	respondError(w, r, 0, "unknown route")
}
