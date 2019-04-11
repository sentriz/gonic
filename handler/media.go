package handler

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"unicode"

	"github.com/jinzhu/gorm"
	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/subsonic"
)

func (c *Controller) Ping(w http.ResponseWriter, req *http.Request) {
	sub := subsonic.NewResponse()
	respond(w, req, sub)
}

func (c *Controller) GetIndexes(w http.ResponseWriter, req *http.Request) {
	var artists []db.Artist
	c.DB.Find(&artists)
	indexMap := make(map[byte]*subsonic.Index)
	for _, artist := range artists {
		first := artist.Name[0]
		if !unicode.IsLetter(rune(first)) {
			first = 0x23 // '#'
		}
		_, ok := indexMap[first]
		if !ok {
			indexMap[first] = &subsonic.Index{
				Name:    string(first),
				Artists: []*subsonic.Artist{},
			}
		}
		indexMap[first].Artists = append(
			indexMap[first].Artists,
			&subsonic.Artist{
				ID:   artist.ID,
				Name: artist.Name,
			},
		)
	}
	indexes := []*subsonic.Index{}
	for _, v := range indexMap {
		indexes = append(indexes, v)
	}
	sub := subsonic.NewResponse()
	sub.Indexes = &subsonic.Indexes{
		Index: &indexes,
	}
	respond(w, req, sub)
}

func browseArtist(c *gorm.DB, artist *db.Artist) *subsonic.Directory {
	var cover db.Cover
	var dir subsonic.Directory
	dir.Name = artist.Name
	dir.ID = artist.ID
	dir.Parent = 0
	var albums []*db.Album
	c.Model(artist).Related(&albums)
	dir.Children = make([]subsonic.Child, len(albums))
	for i, album := range albums {
		c.Model(album).Related(&cover)
		dir.Children[i] = subsonic.Child{
			Artist:   artist.Name,
			ID:       album.ID,
			IsDir:    true,
			Parent:   artist.ID,
			Title:    album.Title,
			CoverArt: cover.ID,
		}
		cover = db.Cover{}
	}
	return &dir
}

func browseAlbum(c *gorm.DB, album *db.Album) *subsonic.Directory {
	var artist db.Artist
	c.Model(album).Related(&artist)
	var tracks []*db.Track
	c.Model(album).Related(&tracks)
	var cover db.Cover
	c.Model(album).Related(&cover)
	var dir subsonic.Directory
	dir.Name = album.Title
	dir.ID = album.ID
	dir.Parent = artist.ID
	dir.Children = make([]subsonic.Child, len(tracks))
	for i, track := range tracks {
		dir.Children[i] = subsonic.Child{
			ID:          track.ID,
			Title:       track.Title,
			Parent:      album.ID,
			Artist:      artist.Name,
			ArtistID:    artist.ID,
			Album:       album.Title,
			AlbumID:     album.ID,
			IsDir:       false,
			Path:        track.Path,
			CoverArt:    cover.ID,
			ContentType: track.ContentType,
			Suffix:      track.Suffix,
			Duration:    0,
		}
	}
	return &dir
}

func (c *Controller) GetMusicDirectory(w http.ResponseWriter, req *http.Request) {
	idStr := req.URL.Query().Get("id")
	if idStr == "" {
		respondError(w, req, 10, "please provide an `id` parameter")
		return
	}
	id, _ := strconv.Atoi(idStr)
	sub := subsonic.NewResponse()
	var artist db.Artist
	c.DB.First(&artist, id)
	if artist.ID != 0 {
		sub.MusicDirectory = browseArtist(c.DB, &artist)
		respond(w, req, sub)
		return
	}
	var album db.Album
	c.DB.First(&album, id)
	if album.ID != 0 {
		sub.MusicDirectory = browseAlbum(c.DB, &album)
		respond(w, req, sub)
		return
	}
	respondError(w, req,
		70, fmt.Sprintf("directory with id `%d` was not found", id),
	)
}

func (c *Controller) GetCoverArt(w http.ResponseWriter, req *http.Request) {
	idStr := req.URL.Query().Get("id")
	if idStr == "" {
		respondError(w, req, 10, "please provide an `id` parameter")
		return
	}
	id, _ := strconv.Atoi(idStr)
	var cover db.Cover
	c.DB.First(&cover, id)
	w.Write(cover.Image)
}

func (c *Controller) Stream(w http.ResponseWriter, req *http.Request) {
	idStr := req.URL.Query().Get("id")
	if idStr == "" {
		respondError(w, req, 10, "please provide an `id` parameter")
		return
	}
	id, _ := strconv.Atoi(idStr)
	var track db.Track
	c.DB.First(&track, id)
	if track.Path == "" {
		respondError(w, req, 70, fmt.Sprintf("media with id `%d` was not found", id))
		return
	}
	file, err := os.Open(track.Path)
	if err != nil {
		respondError(w, req, 0, fmt.Sprintf("error while streaming media: %v", err))
		return
	}
	stat, _ := file.Stat()
	http.ServeContent(w, req, track.Path, stat.ModTime(), file)
}

func (c *Controller) GetLicence(w http.ResponseWriter, req *http.Request) {
	sub := subsonic.NewResponse()
	sub.Licence = &subsonic.Licence{
		Valid: true,
	}
	respond(w, req, sub)
}

func (c *Controller) NotFound(w http.ResponseWriter, req *http.Request) {
	respondError(w, req, 0, "unknown route")
}
