package ctrlsubsonic

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/jinzhu/gorm"

	"senan.xyz/g/gonic/model"
	"senan.xyz/g/gonic/server/ctrlsubsonic/params"
	"senan.xyz/g/gonic/server/ctrlsubsonic/spec"
)

func (c *Controller) ServeGetArtists(r *http.Request) *spec.Response {
	var artists []*model.Artist
	c.DB.
		Select("*, count(sub.id) as album_count").
		Joins(`
            LEFT JOIN albums sub
		    ON artists.id = sub.tag_artist_id
		`).
		Group("artists.id").
		Find(&artists)
	// [a-z#] -> 27
	indexMap := make(map[string]*spec.Index, 27)
	resp := make([]*spec.Index, 0, 27)
	for _, artist := range artists {
		i := lowerUDecOrHash(artist.IndexName())
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
			spec.NewArtistByTags(artist))
	}
	sort.Slice(resp, func(i, j int) bool {
		return resp[i].Name < resp[j].Name
	})
	sub := spec.NewResponse()
	sub.Artists = &spec.Artists{
		List: resp,
	}
	return sub
}

func (c *Controller) ServeGetArtist(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetInt("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	artist := &model.Artist{}
	c.DB.
		Preload("Albums").
		First(artist, id)
	sub := spec.NewResponse()
	sub.Artist = spec.NewArtistByTags(artist)
	sub.Artist.Albums = make([]*spec.Album, len(artist.Albums))
	for i, album := range artist.Albums {
		sub.Artist.Albums[i] = spec.NewAlbumByTags(album, artist)
	}
	sub.Artist.AlbumCount = len(artist.Albums)
	return sub
}

func (c *Controller) ServeGetAlbum(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetInt("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	album := &model.Album{}
	err = c.DB.
		Preload("TagArtist").
		Preload("Tracks", func(db *gorm.DB) *gorm.DB {
			return db.Order("tracks.tag_disc_number, tracks.tag_track_number")
		}).
		First(album, id).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return spec.NewError(10, "couldn't find an album with that id")
	}
	sub := spec.NewResponse()
	sub.Album = spec.NewAlbumByTags(album, album.TagArtist)
	sub.Album.Tracks = make([]*spec.TrackChild, len(album.Tracks))
	for i, track := range album.Tracks {
		sub.Album.Tracks[i] = spec.NewTrackByTags(track, album)
	}
	return sub
}

// changes to this function should be reflected in in _by_folder.go's
// getAlbumList() function
func (c *Controller) ServeGetAlbumListTwo(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	listType := params.Get("type")
	if listType == "" {
		return spec.NewError(10, "please provide a `type` parameter")
	}
	q := c.DB.DB
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
			params.GetIntOr("fromYear", 1800),
			params.GetIntOr("toYear", 2200))
		q = q.Order("tag_year")
	case "frequent":
		user := r.Context().Value(CtxUser).(*model.User)
		q = q.Joins(`
			JOIN plays
			ON albums.id = plays.album_id AND plays.user_id = ?`,
			user.ID)
		q = q.Order("plays.count DESC")
	case "newest":
		q = q.Order("modified_at DESC")
	case "random":
		q = q.Order(gorm.Expr("random()"))
	case "recent":
		user := r.Context().Value(CtxUser).(*model.User)
		q = q.Joins(`
			JOIN plays
			ON albums.id = plays.album_id AND plays.user_id = ?`,
			user.ID)
		q = q.Order("plays.time DESC")
	default:
		return spec.NewError(10, "unknown value `%s` for parameter 'type'", listType)
	}
	var albums []*model.Album
	q.
		Where("albums.tag_artist_id IS NOT NULL").
		Offset(params.GetIntOr("offset", 0)).
		Limit(params.GetIntOr("size", 10)).
		Preload("TagArtist").
		Find(&albums)
	sub := spec.NewResponse()
	sub.AlbumsTwo = &spec.Albums{
		List: make([]*spec.Album, len(albums)),
	}
	for i, album := range albums {
		sub.AlbumsTwo.List[i] = spec.NewAlbumByTags(album, album.TagArtist)
	}
	return sub
}

func (c *Controller) ServeSearchThree(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	query := params.Get("query")
	if query == "" {
		return spec.NewError(10, "please provide a `query` parameter")
	}
	query = fmt.Sprintf("%%%s%%",
		strings.TrimSuffix(query, "*"))
	results := &spec.SearchResultThree{}
	//
	// search "artists"
	var artists []*model.Artist
	c.DB.
		Where(`
            name LIKE ? OR
            name_u_dec LIKE ?
		`, query, query).
		Offset(params.GetIntOr("artistOffset", 0)).
		Limit(params.GetIntOr("artistCount", 20)).
		Find(&artists)
	for _, a := range artists {
		results.Artists = append(results.Artists,
			spec.NewArtistByTags(a))
	}
	//
	// search "albums"
	var albums []*model.Album
	c.DB.
		Preload("TagArtist").
		Where(`
            tag_title LIKE ? OR
            tag_title_u_dec LIKE ?
		`, query, query).
		Offset(params.GetIntOr("albumOffset", 0)).
		Limit(params.GetIntOr("albumCount", 20)).
		Find(&albums)
	for _, a := range albums {
		results.Albums = append(results.Albums,
			spec.NewAlbumByTags(a, a.TagArtist))
	}
	//
	// search tracks
	var tracks []*model.Track
	c.DB.
		Preload("Album").
		Where(`
            tag_title LIKE ? OR
            tag_title_u_dec LIKE ?
		`, query, query).
		Offset(params.GetIntOr("songOffset", 0)).
		Limit(params.GetIntOr("songCount", 20)).
		Find(&tracks)
	for _, t := range tracks {
		results.Tracks = append(results.Tracks,
			spec.NewTrackByTags(t, t.Album))
	}
	sub := spec.NewResponse()
	sub.SearchResultThree = results
	return sub
}
