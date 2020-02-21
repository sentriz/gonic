package ctrlsubsonic

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/jinzhu/gorm"

	"senan.xyz/g/gonic/db"
	"senan.xyz/g/gonic/server/ctrlsubsonic/params"
	"senan.xyz/g/gonic/server/ctrlsubsonic/spec"
	"senan.xyz/g/gonic/server/lastfm"
)

func (c *Controller) ServeGetArtists(r *http.Request) *spec.Response {
	var artists []*db.Artist
	c.DB.
		Select("*, count(sub.id) album_count").
		Joins("JOIN albums sub ON artists.id=sub.tag_artist_id").
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
	artist := &db.Artist{}
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
	album := &db.Album{}
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
		q = q.Joins("JOIN artists ON albums.tag_artist_id=artists.id")
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
		user := r.Context().Value(CtxUser).(*db.User)
		q = q.Joins("JOIN plays ON albums.id=plays.album_id AND plays.user_id=?",
			user.ID)
		q = q.Order("plays.count DESC")
	case "newest":
		q = q.Order("modified_at DESC")
	case "random":
		q = q.Order(gorm.Expr("random()"))
	case "recent":
		user := r.Context().Value(CtxUser).(*db.User)
		q = q.Joins("JOIN plays ON albums.id=plays.album_id AND plays.user_id=?",
			user.ID)
		q = q.Order("plays.time DESC")
	default:
		return spec.NewError(10, "unknown value `%s` for parameter 'type'", listType)
	}
	var albums []*db.Album
	// TODO: think about removing this extra join to count number
	// of children. it might make sense to store that in the db
	q.
		Select("albums.*, count(tracks.id) child_count").
		Joins("JOIN tracks ON tracks.album_id=albums.id").
		Group("albums.id").
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
	var artists []*db.Artist
	c.DB.
		Where("name LIKE ? OR name_u_dec LIKE ?",
			query, query).
		Offset(params.GetIntOr("artistOffset", 0)).
		Limit(params.GetIntOr("artistCount", 20)).
		Find(&artists)
	for _, a := range artists {
		results.Artists = append(results.Artists,
			spec.NewArtistByTags(a))
	}
	//
	// search "albums"
	var albums []*db.Album
	c.DB.
		Preload("TagArtist").
		Where("tag_title LIKE ? OR tag_title_u_dec LIKE ?",
			query, query).
		Offset(params.GetIntOr("albumOffset", 0)).
		Limit(params.GetIntOr("albumCount", 20)).
		Find(&albums)
	for _, a := range albums {
		results.Albums = append(results.Albums,
			spec.NewAlbumByTags(a, a.TagArtist))
	}
	//
	// search tracks
	var tracks []*db.Track
	c.DB.
		Preload("Album").
		Where("tag_title LIKE ? OR tag_title_u_dec LIKE ?",
			query, query).
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

func (c *Controller) ServeGetArtistInfoTwo(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetInt("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	apiKey := c.DB.GetSetting("lastfm_api_key")
	if apiKey == "" {
		return spec.NewError(0, "please set ask your admin to set the last.fm api key")
	}
	artist := &db.Artist{}
	err = c.DB.
		Where("id=?", id).
		Find(artist).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return spec.NewError(70, "artist with id `%d` not found", id)
	}
	info, err := lastfm.ArtistGetInfo(apiKey, artist)
	if err != nil {
		return spec.NewError(0, "fetching artist info: %v", err)
	}
	sub := spec.NewResponse()
	sub.ArtistInfoTwo = &spec.ArtistInfo{
		Biography:     info.Bio.Summary,
		MusicBrainzID: info.MBID,
		LastFMURL:     info.URL,
	}
	for _, image := range info.Image {
		switch image.Size {
		case "small":
			sub.ArtistInfoTwo.SmallImageURL = image.Text
		case "medium":
			sub.ArtistInfoTwo.MediumImageURL = image.Text
		case "large":
			sub.ArtistInfoTwo.LargeImageURL = image.Text
		}
	}
	count := params.GetIntOr("count", 20)
	includeNotPresent := params.Get("includeNotPresent") == "true"
	for i, similarInfo := range info.Similar.Artists {
		if i == count {
			break
		}
		artist = &db.Artist{}
		err = c.DB.
			Select("artists.*, count(albums.id) album_count").
			Where("name=?", similarInfo.Name).
			Joins("JOIN albums ON artists.id=albums.tag_artist_id").
			Group("artists.id").
			Find(artist).
			Error
		if gorm.IsRecordNotFoundError(err) && !includeNotPresent {
			continue
		}
		similar := &spec.SimilarArtist{ID: -1}
		if artist.ID != 0 {
			similar.ID = artist.ID
		}
		similar.Name = similarInfo.Name
		similar.AlbumCount = artist.AlbumCount
		sub.ArtistInfoTwo.SimilarArtist = append(
			sub.ArtistInfoTwo.SimilarArtist, similar)
	}
	return sub
}
